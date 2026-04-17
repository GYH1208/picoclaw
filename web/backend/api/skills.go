package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/skills"
	"gopkg.in/yaml.v3"
)

type skillSupportResponse struct {
	Skills []skills.SkillInfo `json:"skills"`
}

type skillDetailResponse struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type importedSkillResponse struct {
	Name                  string   `json:"name"`
	Title                 string   `json:"title,omitempty"`
	Path                  string   `json:"path"`
	Source                string   `json:"source"`
	Description           string   `json:"description"`
	Warnings              []string `json:"warnings,omitempty"`
	DocumentationWarnings []string `json:"documentationWarnings,omitempty"`
	ReviewWarnings        []string `json:"reviewWarnings,omitempty"`
}

type importedSkillScanSummary struct {
	Warnings              []string
	DocumentationWarnings []string
	ReviewWarnings        []string
}

var (
	skillNameSanitizer            = regexp.MustCompile(`[^a-z0-9-]+`)
	importedSkillFrontmatter      = regexp.MustCompile(`(?s)^---(?:\r\n|\n|\r)(.*?)(?:\r\n|\n|\r)---(?:\r\n|\n|\r)*`)
	skillFrontmatterStripper      = regexp.MustCompile(`(?s)^---(?:\r\n|\n|\r)(.*?)(?:\r\n|\n|\r)---(?:\r\n|\n|\r)*`)
	importedSkillBlockingPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{
			name:    "remote_shell_pipe",
			pattern: regexp.MustCompile(`(?i)\b(?:curl|wget)\b[\s\S]{0,200}\|\s*(?:sh|bash)\b`),
		},
		{
			name:    "reverse_shell",
			pattern: regexp.MustCompile(`(?i)\b(?:nc\s+-e|bash\s+-i\b|sh\s+-i\b|/dev/tcp/|powershell\s+-enc\b)`),
		},
		{
			name:    "destructive_command",
			pattern: regexp.MustCompile(`(?i)\b(?:rm\s+-rf\b|dd\s+if=|mkfs\b|format\s+c:)\b`),
		},
	}
	importedSkillWarningPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{
			name:    "credential_access",
			pattern: regexp.MustCompile(`(?i)(/etc/shadow|/root/|~/.ssh|authorized_keys|id_rsa|aws_access_key_id|api[_-]?key|secret[_-]?key)`),
		},
		{
			name:    "privileged_command",
			pattern: regexp.MustCompile(`(?i)\b(?:sudo|chmod|chown|systemctl|service)\b`),
		},
		{
			name:    "network_fetch",
			pattern: regexp.MustCompile(`(?i)\b(?:curl|wget|ssh|scp)\b`),
		},
		{
			name:    "process_control",
			pattern: regexp.MustCompile(`(?i)\b(?:pkill|killall|kill\s+-9)\b`),
		},
	}
)

const (
	importedSkillFilename   = "SKILL.md"
	importedSkillField      = "file"
	importedSkillFilesField = "files"
	importedSkillPathsField = "paths"
	importedSkillMaxBytes   = 8 << 20
	importedSkillMaxFiles   = 128
)

var importedSkillBlockedExtensions = map[string]struct{}{
	".sh":    {},
	".bash":  {},
	".zsh":   {},
	".ps1":   {},
	".bat":   {},
	".cmd":   {},
	".exe":   {},
	".dll":   {},
	".so":    {},
	".dylib": {},
	".msi":   {},
	".com":   {},
	".jar":   {},
}

func (h *Handler) registerSkillRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/skills", h.handleListSkills)
	mux.HandleFunc("GET /api/skills/{name}", h.handleGetSkill)
	mux.HandleFunc("POST /api/skills/import", h.handleImportSkill)
	mux.HandleFunc("DELETE /api/skills/{name}", h.handleDeleteSkill)
}

func (h *Handler) handleListSkills(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	loader := newSkillsLoader(cfg.WorkspacePath())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(skillSupportResponse{
		Skills: loader.ListSkills(),
	})
}

func (h *Handler) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	loader := newSkillsLoader(cfg.WorkspacePath())
	name := r.PathValue("name")
	allSkills := loader.ListSkills()

	for _, skill := range allSkills {
		if skill.Name != name {
			continue
		}

		content, err := loadSkillContent(skill.Path)
		if err != nil {
			http.Error(w, "Skill content not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(skillDetailResponse{
			Name:        skill.Name,
			Title:       skill.Title,
			Path:        skill.Path,
			Source:      skill.Source,
			Description: skill.Description,
			Content:     content,
		})
		return
	}

	http.Error(w, "Skill not found", http.StatusNotFound)
}

func (h *Handler) handleImportSkill(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	err = r.ParseMultipartForm(4 << 20)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid multipart form: %v", err), http.StatusBadRequest)
		return
	}

	importedFiles, err := readImportedSkillPackage(r.MultipartForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	skillPath, packageRoot, err := locateImportedSkillRoot(importedFiles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	skillContent, ok := importedFiles[skillPath]
	if !ok {
		http.Error(w, "SKILL.md not found in package", http.StatusBadRequest)
		return
	}

	skillName, description, err := normalizeImportedSkillMetadata(skillContent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	importedFiles = rebaseImportedSkillFiles(importedFiles, packageRoot)
	importedFiles[importedSkillFilename] = normalizeImportedSkillContent(skillContent, skillName, description)
	scanSummary, err := scanImportedSkillFiles(importedFiles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	workspace := cfg.WorkspacePath()
	skillDir := filepath.Join(workspace, "skills", skillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillDir); err == nil {
		http.Error(w, "skill already exists", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create skill directory: %v", err), http.StatusInternalServerError)
		return
	}

	writeErr := writeImportedSkillFiles(skillDir, importedFiles)
	if writeErr != nil {
		_ = os.RemoveAll(skillDir)
		http.Error(w, fmt.Sprintf("Failed to save skill: %v", writeErr), http.StatusInternalServerError)
		return
	}

	loader := newSkillsLoader(workspace)
	for _, skill := range loader.ListSkills() {
		if skill.Path == skillFile || (skill.Name == skillName && skill.Source == "workspace") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(importedSkillResponse{
				Name:                  skill.Name,
				Title:                 skill.Title,
				Path:                  skill.Path,
				Source:                skill.Source,
				Description:           skill.Description,
				Warnings:              scanSummary.Warnings,
				DocumentationWarnings: scanSummary.DocumentationWarnings,
				ReviewWarnings:        scanSummary.ReviewWarnings,
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name":                  skillName,
		"path":                  skillFile,
		"warnings":              scanSummary.Warnings,
		"documentationWarnings": scanSummary.DocumentationWarnings,
		"reviewWarnings":        scanSummary.ReviewWarnings,
	})
}

func (h *Handler) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	loader := newSkillsLoader(cfg.WorkspacePath())
	name := r.PathValue("name")
	for _, skill := range loader.ListSkills() {
		if skill.Name != name {
			continue
		}
		if skill.Source != "workspace" {
			http.Error(w, "only workspace skills can be deleted", http.StatusBadRequest)
			return
		}
		if err := os.RemoveAll(filepath.Dir(skill.Path)); err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete skill: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	http.Error(w, "Skill not found", http.StatusNotFound)
}

func newSkillsLoader(workspace string) *skills.SkillsLoader {
	return skills.NewSkillsLoader(
		workspace,
		filepath.Join(globalConfigDir(), "skills"),
		builtinSkillsDir(),
	)
}

func normalizeImportedSkillName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ToLower(raw)
	raw = strings.ReplaceAll(raw, "_", "-")
	raw = strings.ReplaceAll(raw, " ", "-")
	raw = skillNameSanitizer.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	raw = strings.Join(strings.FieldsFunc(raw, func(r rune) bool { return r == '-' }), "-")

	if raw == "" {
		return "", fmt.Errorf("frontmatter name is required")
	}
	if len(raw) > skills.MaxNameLength {
		return "", fmt.Errorf("skill name exceeds %d characters", skills.MaxNameLength)
	}
	matched, err := regexp.MatchString(`^[a-z0-9]+(-[a-z0-9]+)*$`, raw)
	if err != nil || !matched {
		return "", fmt.Errorf("skill name must be alphanumeric with hyphens")
	}
	return raw, nil
}

func normalizeImportedSkillMetadata(content []byte) (string, string, error) {
	rawContent := strings.ReplaceAll(string(content), "\r\n", "\n")
	rawContent = strings.ReplaceAll(rawContent, "\r", "\n")
	metadata, _ := extractImportedSkillMetadata(rawContent)

	raw := strings.TrimSpace(metadata["name"])
	skillName, err := normalizeImportedSkillName(raw)
	if err != nil {
		return "", "", err
	}
	description := strings.TrimSpace(metadata["description"])
	if description == "" {
		return "", "", fmt.Errorf("frontmatter description is required")
	}
	if len(description) > skills.MaxDescriptionLength {
		return "", "", fmt.Errorf("description exceeds %d characters", skills.MaxDescriptionLength)
	}
	return skillName, description, nil
}

func normalizeImportedSkillContent(content []byte, skillName, description string) []byte {
	raw := strings.ReplaceAll(string(content), "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	metadata, body := extractImportedSkillMetadata(raw)
	title := strings.TrimSpace(metadata["title"])

	body = strings.TrimLeft(body, "\n")
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("name: ")
	builder.WriteString(skillName)
	builder.WriteString("\n")
	builder.WriteString("description: ")
	builder.WriteString(description)
	builder.WriteString("\n")
	if title != "" {
		builder.WriteString("title: ")
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	builder.WriteString("---\n\n")
	builder.WriteString(body)
	if !strings.HasSuffix(builder.String(), "\n") {
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func extractImportedSkillMetadata(raw string) (map[string]string, string) {
	matches := importedSkillFrontmatter.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return map[string]string{}, raw
	}
	meta := parseImportedSkillYAML(matches[1])
	body := importedSkillFrontmatter.ReplaceAllString(raw, "")
	return meta, body
}

func parseImportedSkillYAML(frontmatter string) map[string]string {
	result := make(map[string]string)
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Title       string `yaml:"title"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return result
	}
	if meta.Name != "" {
		result["name"] = strings.TrimSpace(meta.Name)
	}
	if meta.Description != "" {
		result["description"] = strings.TrimSpace(meta.Description)
	}
	if meta.Title != "" {
		result["title"] = strings.TrimSpace(meta.Title)
	}
	return result
}

func readImportedSkillPackage(form *multipart.Form) (map[string][]byte, error) {
	if form == nil {
		return nil, fmt.Errorf("file is required")
	}
	if files := form.File[importedSkillFilesField]; len(files) > 0 {
		return readImportedSkillFolder(files, form.Value[importedSkillPathsField])
	}
	if files := form.File[importedSkillField]; len(files) > 0 {
		if len(files) != 1 {
			return nil, fmt.Errorf("exactly one file upload is allowed")
		}
		return readImportedSingleUpload(files[0])
	}
	return nil, fmt.Errorf("file is required")
}

func readImportedSingleUpload(fileHeader *multipart.FileHeader) (map[string][]byte, error) {
	filename := filepath.Base(strings.TrimSpace(fileHeader.Filename))
	switch {
	case filename == importedSkillFilename:
		content, err := readLimitedMultipartFile(fileHeader, importedSkillMaxBytes)
		if err != nil {
			return nil, err
		}
		return map[string][]byte{importedSkillFilename: content}, nil
	case strings.EqualFold(filepath.Ext(filename), ".zip"):
		content, err := readLimitedMultipartFile(fileHeader, importedSkillMaxBytes)
		if err != nil {
			return nil, err
		}
		return readImportedZip(content)
	default:
		return nil, fmt.Errorf("invalid upload: only %s or .zip is allowed", importedSkillFilename)
	}
}

func readImportedSkillFolder(fileHeaders []*multipart.FileHeader, relPaths []string) (map[string][]byte, error) {
	if len(fileHeaders) > importedSkillMaxFiles {
		return nil, fmt.Errorf("too many files in skill package")
	}
	if len(relPaths) > 0 && len(relPaths) != len(fileHeaders) {
		return nil, fmt.Errorf("invalid folder upload metadata")
	}
	files := make(map[string][]byte, len(fileHeaders))
	totalBytes := 0
	for idx, fileHeader := range fileHeaders {
		pathName := fileHeader.Filename
		if len(relPaths) == len(fileHeaders) {
			pathName = relPaths[idx]
		}
		relPath, err := sanitizeImportedSkillRelativePath(pathName)
		if err != nil {
			return nil, err
		}
		if relPath == "" || isIgnoredImportedSkillPath(relPath) {
			continue
		}
		content, err := readLimitedMultipartFile(fileHeader, importedSkillMaxBytes-totalBytes)
		if err != nil {
			return nil, err
		}
		totalBytes += len(content)
		if totalBytes > importedSkillMaxBytes {
			return nil, fmt.Errorf("skill package exceeds 1MB limit")
		}
		files[relPath] = content
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("skill package is empty")
	}
	return files, nil
}

func readImportedZip(content []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file")
	}
	if len(reader.File) > importedSkillMaxFiles {
		return nil, fmt.Errorf("too many files in skill package")
	}
	files := make(map[string][]byte, len(reader.File))
	totalBytes := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		relPath, err := sanitizeImportedSkillRelativePath(file.Name)
		if err != nil {
			return nil, err
		}
		if relPath == "" || isIgnoredImportedSkillPath(relPath) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to read zip entry: %v", err)
		}
		entry, readErr := io.ReadAll(io.LimitReader(rc, int64(importedSkillMaxBytes-totalBytes)+1))
		_ = rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read zip entry: %v", readErr)
		}
		totalBytes += len(entry)
		if totalBytes > importedSkillMaxBytes {
			return nil, fmt.Errorf("skill package exceeds 1MB limit")
		}
		files[relPath] = entry
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("skill package is empty")
	}
	return files, nil
}

func readLimitedMultipartFile(fileHeader *multipart.FileHeader, remaining int) ([]byte, error) {
	if remaining <= 0 {
		return nil, fmt.Errorf("skill package exceeds 1MB limit")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(remaining)+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	if len(content) > remaining {
		return nil, fmt.Errorf("skill package exceeds 1MB limit")
	}
	return content, nil
}

func sanitizeImportedSkillRelativePath(name string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if normalized == "" {
		return "", nil
	}
	clean := path.Clean(normalized)
	if clean == "." {
		return "", nil
	}
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid path in skill package")
	}
	return clean, nil
}

func isIgnoredImportedSkillPath(relPath string) bool {
	base := path.Base(relPath)
	return strings.HasPrefix(relPath, "__MACOSX/") || base == ".DS_Store"
}

func scanImportedSkillFiles(files map[string][]byte) (importedSkillScanSummary, error) {
	findingsByPath := make(map[string]map[string]struct{})
	for relPath, content := range files {
		ext := strings.ToLower(path.Ext(relPath))
		if _, blocked := importedSkillBlockedExtensions[ext]; blocked {
			return importedSkillScanSummary{}, fmt.Errorf("executable files are not allowed in skill packages: %s", relPath)
		}

		if !isImportedSkillTextFile(relPath) {
			// Binary assets (e.g. images) are allowed; we don't pattern-scan them.
			continue
		}

		if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
			return importedSkillScanSummary{}, fmt.Errorf("only UTF-8 text files are allowed for scanned files: %s", relPath)
		}

		text := string(content)
		scanText := text
		if isImportedSkillMarkdownDoc(relPath) {
			scanText = stripMarkdownCodeFences(text)
		}
		for _, rule := range importedSkillBlockingPatterns {
			if rule.pattern.FindStringIndex(scanText) != nil {
				if path.Base(relPath) == importedSkillFilename {
					return importedSkillScanSummary{}, fmt.Errorf("skill package blocked by lightweight security scan (%s) in %s", rule.name, relPath)
				}
				recordImportedSkillFinding(findingsByPath, relPath, rule.name)
			}
		}
		for _, rule := range importedSkillWarningPatterns {
			if rule.pattern.FindStringIndex(scanText) != nil {
				recordImportedSkillFinding(findingsByPath, relPath, rule.name)
			}
		}
	}
	return summarizeImportedSkillFindings(findingsByPath), nil
}

func isImportedSkillTextFile(relPath string) bool {
	if path.Base(relPath) == importedSkillFilename {
		return true
	}
	switch strings.ToLower(path.Ext(relPath)) {
	case ".md", ".txt", ".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf", ".csv", ".xml":
		return true
	default:
		return false
	}
}

func isImportedSkillMarkdownDoc(relPath string) bool {
	return strings.ToLower(path.Ext(relPath)) == ".md" && path.Base(relPath) != importedSkillFilename
}

func stripMarkdownCodeFences(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func recordImportedSkillFinding(findingsByPath map[string]map[string]struct{}, relPath, code string) {
	if _, ok := findingsByPath[relPath]; !ok {
		findingsByPath[relPath] = make(map[string]struct{})
	}
	findingsByPath[relPath][code] = struct{}{}
}

func summarizeImportedSkillFindings(findingsByPath map[string]map[string]struct{}) importedSkillScanSummary {
	if len(findingsByPath) == 0 {
		return importedSkillScanSummary{}
	}
	orderedPaths := make([]string, 0, len(findingsByPath))
	for relPath := range findingsByPath {
		orderedPaths = append(orderedPaths, relPath)
	}
	slices.Sort(orderedPaths)

	documentationWarnings := make([]string, 0, len(findingsByPath))
	reviewWarnings := make([]string, 0, len(findingsByPath))
	for _, relPath := range orderedPaths {
		codes := findingsByPath[relPath]
		if isImportedSkillMarkdownDoc(relPath) {
			documentationWarnings = append(documentationWarnings, summarizeImportedSkillDocFinding(relPath, codes))
			continue
		}
		reviewWarnings = append(reviewWarnings, summarizeImportedSkillGenericFindings(relPath, codes)...)
	}
	allWarnings := make([]string, 0, len(documentationWarnings)+len(reviewWarnings))
	allWarnings = append(allWarnings, documentationWarnings...)
	allWarnings = append(allWarnings, reviewWarnings...)
	return importedSkillScanSummary{
		Warnings:              allWarnings,
		DocumentationWarnings: documentationWarnings,
		ReviewWarnings:        reviewWarnings,
	}
}

func summarizeImportedSkillDocFinding(relPath string, codes map[string]struct{}) string {
	switch {
	case hasImportedSkillFinding(codes, "credential_access") && len(codes) == 1:
		return fmt.Sprintf("文档中提及敏感凭据或系统路径，请确认未包含真实密钥（%s）", relPath)
	case hasImportedSkillFinding(codes, "destructive_command"),
		hasImportedSkillFinding(codes, "reverse_shell"),
		hasImportedSkillFinding(codes, "remote_shell_pipe"):
		return fmt.Sprintf("文档中包含高风险命令示例，请确认仅用于说明且不会被自动执行（%s）", relPath)
	case hasImportedSkillFinding(codes, "network_fetch"),
		hasImportedSkillFinding(codes, "privileged_command"),
		hasImportedSkillFinding(codes, "process_control"):
		return fmt.Sprintf("文档中包含系统命令或远程访问示例，请确认仅用于说明（%s）", relPath)
	default:
		return fmt.Sprintf("文档中包含需要人工复核的内容，请确认仅用于说明（%s）", relPath)
	}
}

func summarizeImportedSkillGenericFindings(relPath string, codes map[string]struct{}) []string {
	orderedCodes := []string{
		"remote_shell_pipe",
		"reverse_shell",
		"destructive_command",
		"credential_access",
		"privileged_command",
		"network_fetch",
		"process_control",
	}
	labelMap := map[string]string{
		"remote_shell_pipe":   "文件中包含远程下载后直接执行的命令示例，请确认仅用于说明",
		"reverse_shell":       "文件中包含疑似远程控制命令示例，请人工复核",
		"destructive_command": "文件中包含破坏性命令示例，请人工复核",
		"credential_access":   "文件中提及敏感凭据或系统路径，请确认未包含真实密钥",
		"privileged_command":  "文件中包含提权或服务管理命令，请确认仅用于说明",
		"network_fetch":       "文件中包含网络下载或远程访问命令，请确认仅用于说明",
		"process_control":     "文件中包含进程控制命令，请确认仅用于说明",
	}
	summaries := make([]string, 0, len(codes))
	for _, code := range orderedCodes {
		if !hasImportedSkillFinding(codes, code) {
			continue
		}
		summaries = append(summaries, fmt.Sprintf("%s（%s）", labelMap[code], relPath))
	}
	return summaries
}

func hasImportedSkillFinding(codes map[string]struct{}, code string) bool {
	_, ok := codes[code]
	return ok
}

func locateImportedSkillRoot(files map[string][]byte) (string, string, error) {
	var matches []string
	for relPath := range files {
		if path.Base(relPath) == importedSkillFilename {
			matches = append(matches, relPath)
		}
	}
	if len(matches) == 0 {
		return "", "", fmt.Errorf("skill package must contain %s", importedSkillFilename)
	}
	if len(matches) > 1 {
		return "", "", fmt.Errorf("skill package must contain exactly one %s", importedSkillFilename)
	}
	skillPath := matches[0]
	root := path.Dir(skillPath)
	if root == "." {
		root = ""
	}
	return skillPath, root, nil
}

func rebaseImportedSkillFiles(files map[string][]byte, root string) map[string][]byte {
	if root == "" {
		return files
	}
	rebased := make(map[string][]byte, len(files))
	prefix := root + "/"
	for relPath, content := range files {
		if relPath == root || !strings.HasPrefix(relPath, prefix) {
			continue
		}
		rebased[strings.TrimPrefix(relPath, prefix)] = content
	}
	return rebased
}

func writeImportedSkillFiles(skillDir string, files map[string][]byte) error {
	for relPath, content := range files {
		targetPath := filepath.Join(skillDir, filepath.FromSlash(relPath))
		parent := filepath.Dir(targetPath)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadSkillContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return skillFrontmatterStripper.ReplaceAllString(string(content), ""), nil
}

func globalConfigDir() string {
	return config.GetHome()
}

func builtinSkillsDir() string {
	if path := os.Getenv(config.EnvBuiltinSkills); path != "" {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(wd, "skills")
}
