import {
  IconAlertTriangle,
  IconChevronDown,
  IconChevronUp,
  IconCheck,
  IconFileInfo,
  IconLoader2,
  IconPlus,
  IconTrash,
  IconUpload,
  IconX,
} from "@tabler/icons-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { type ChangeEvent, type DragEvent, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import ReactMarkdown from "react-markdown"
import rehypeRaw from "rehype-raw"
import rehypeSanitize from "rehype-sanitize"
import remarkGfm from "remark-gfm"
import { toast } from "sonner"

import {
  type SkillSupportItem,
  deleteSkill,
  getSkill,
  getSkills,
  importSkill,
} from "@/api/skills"
import { PageHeader } from "@/components/page-header"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

function useSkillCatalogLabels() {
  const { t, i18n } = useTranslation()
  return (skill: SkillSupportItem) => {
    const id = skill.name.toLowerCase()
    const titleKey = `pages.agent.skills.catalog.${id}.title`
    const descKey = `pages.agent.skills.catalog.${id}.description`
    const title = i18n.exists(titleKey)
      ? t(titleKey)
      : (skill.title ?? skill.name)
    const description = i18n.exists(descKey)
      ? t(descKey)
      : skill.description || t("pages.agent.skills.no_description")
    return { title, description }
  }
}

export function SkillsPage() {
  const { t } = useTranslation()
  const skillLabels = useSkillCatalogLabels()
  const queryClient = useQueryClient()
  const importInputRef = useRef<HTMLInputElement | null>(null)
  const folderImportInputRef = useRef<HTMLInputElement | null>(null)
  const [uploadModalOpen, setUploadModalOpen] = useState(false)
  const [dropActive, setDropActive] = useState(false)
  const [autoInstallLowRisk, setAutoInstallLowRisk] = useState(false)
  const [importFeedback, setImportFeedback] = useState<{
    name?: string
    warnings: string[]
    documentationWarnings: string[]
    reviewWarnings: string[]
  } | null>(null)
  const [docWarningsExpanded, setDocWarningsExpanded] = useState(false)
  const [selectedSkill, setSelectedSkill] = useState<SkillSupportItem | null>(
    null,
  )
  const [skillPendingDelete, setSkillPendingDelete] =
    useState<SkillSupportItem | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ["skills"],
    queryFn: getSkills,
  })
  const {
    data: selectedSkillDetail,
    isLoading: isSkillDetailLoading,
    error: skillDetailError,
  } = useQuery({
    queryKey: ["skills", selectedSkill?.name],
    queryFn: () => getSkill(selectedSkill!.name),
    enabled: selectedSkill !== null,
  })

  const importMutation = useMutation({
    mutationFn: async (input: { file?: File; files?: File[] }) => importSkill(input),
    onSuccess: (result) => {
      setDocWarningsExpanded(false)
      setImportFeedback({
        name: result.name,
        warnings: result.warnings ?? [],
        documentationWarnings: result.documentationWarnings ?? [],
        reviewWarnings: result.reviewWarnings ?? [],
      })
      if (result.warnings?.length) {
        toast.warning("技能已导入，检测结果已显示在下方")
      } else {
        toast.success(t("pages.agent.skills.import_success"))
      }
      void queryClient.invalidateQueries({ queryKey: ["skills"] })
    },
    onError: (err) => {
      setDocWarningsExpanded(false)
      setImportFeedback(null)
      toast.error(
        err instanceof Error
          ? err.message
          : t("pages.agent.skills.import_error"),
      )
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (name: string) => deleteSkill(name),
    onSuccess: (_, deletedName) => {
      toast.success(t("pages.agent.skills.delete_success"))
      setSkillPendingDelete(null)
      if (
        selectedSkill?.name === deletedName &&
        selectedSkill.source === "workspace"
      ) {
        setSelectedSkill(null)
      }
      void queryClient.invalidateQueries({ queryKey: ["skills"] })
    },
    onError: (err) => {
      toast.error(
        err instanceof Error
          ? err.message
          : t("pages.agent.skills.delete_error"),
      )
    },
  })

  const handleImportClick = () => {
    setDocWarningsExpanded(false)
    setImportFeedback(null)
    importInputRef.current?.click()
  }

  const handleFolderImportClick = () => {
    setDocWarningsExpanded(false)
    setImportFeedback(null)
    folderImportInputRef.current?.click()
  }

  const handleImportFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    const lowerName = file.name.toLowerCase()
    if (
      lowerName !== "skill.md" &&
      !lowerName.endsWith(".zip")
    ) {
      toast.error("Only SKILL.md or .zip is supported")
      event.target.value = ""
      return
    }
    importMutation.mutate({ file })
    event.target.value = ""
  }

  const handleFolderImportChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    if (files.length === 0) return
    const hasSkillFile = files.some(
      (file) =>
        file.name === "SKILL.md" ||
        file.webkitRelativePath.endsWith("/SKILL.md"),
    )
    if (!hasSkillFile) {
      toast.error("Folder upload must include SKILL.md")
      event.target.value = ""
      return
    }
    importMutation.mutate({ files })
    event.target.value = ""
  }

  const handleDropZoneClick = () => {
    if (importMutation.isPending) return
    handleImportClick()
  }

  const handleDropZoneDragOver = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    if (!importMutation.isPending) {
      setDropActive(true)
    }
  }

  const handleDropZoneDragLeave = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setDropActive(false)
  }

  const handleDropZoneDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setDropActive(false)
    if (importMutation.isPending) return
    const file = event.dataTransfer.files?.[0]
    if (!file) return
    const lowerName = file.name.toLowerCase()
    if (lowerName !== "skill.md" && !lowerName.endsWith(".zip")) {
      toast.error("拖拽上传仅支持 SKILL.md 或 .zip，文件夹请使用“上传文件夹”")
      return
    }
    setImportFeedback(null)
    importMutation.mutate({ file })
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("navigation.skills")}
        children={
          <>
            <input
              ref={importInputRef}
              type="file"
              accept=".md,.zip,text/markdown,text/plain,application/zip"
              className="hidden"
              onChange={handleImportFileChange}
            />
            <input
              ref={folderImportInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={handleFolderImportChange}
              {...({ webkitdirectory: "", directory: "" } as Record<string, string>)}
            />
            <Button
              onClick={() => setUploadModalOpen(true)}
              disabled={importMutation.isPending}
            >
              {importMutation.isPending ? (
                <IconLoader2 className="size-4 animate-spin" />
              ) : (
                <IconPlus className="size-4" />
              )}
              添加技能
            </Button>
          </>
        }
      />

      <div className="flex-1 overflow-auto px-6 py-3">
        <div className="w-full max-w-6xl space-y-6">
          {isLoading ? (
            <div className="text-muted-foreground py-6 text-sm">
              {t("labels.loading")}
            </div>
          ) : error ? (
            <div className="text-destructive py-6 text-sm">
              {t("pages.agent.load_error")}
            </div>
          ) : (
            <section className="space-y-5">
              <p className="text-muted-foreground text-sm">
                {t("pages.agent.skills.description")}
              </p>

              <Card className="border-dashed">
                <CardContent className="space-y-3 py-5">
                  <div className="text-sm font-medium">文件要求</div>
                  <ul className="text-muted-foreground list-disc space-y-1 pl-5 text-sm">
                    <li>单文件上传仅支持 `SKILL.md` 或 `.zip`。</li>
                    <li>文件夹或 `.zip` 必须包含 `SKILL.md`。</li>
                    <li>`SKILL.md` 必须包含 YAML frontmatter，且至少包含 `name` 和 `description`。</li>
                  </ul>
                </CardContent>
              </Card>

              {data?.skills.length ? (
                <div className="grid gap-4 lg:grid-cols-2">
                  {data.skills.map((skill) => {
                    const { title: cardTitle, description: cardDesc } =
                      skillLabels(skill)
                    return (
                      <Card
                        key={`${skill.source}:${skill.name}`}
                        className="border-border/60 gap-4"
                        size="sm"
                      >
                        <CardHeader>
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <CardTitle className="font-semibold">
                                {cardTitle}
                              </CardTitle>
                              <CardDescription className="mt-3">
                                {cardDesc}
                              </CardDescription>
                            </div>
                            <div className="flex items-center gap-1">
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                className="text-muted-foreground hover:text-foreground"
                                onClick={() => setSelectedSkill(skill)}
                                title={t("pages.agent.skills.view")}
                              >
                                <IconFileInfo className="size-4" />
                              </Button>
                              {skill.source === "workspace" ? (
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  className="text-muted-foreground hover:text-destructive"
                                  onClick={() => setSkillPendingDelete(skill)}
                                  title={t("pages.agent.skills.delete")}
                                >
                                  <IconTrash className="size-4" />
                                </Button>
                              ) : null}
                            </div>
                          </div>
                        </CardHeader>
                        <CardContent className="space-y-2">
                          <div className="text-muted-foreground text-[11px] tracking-[0.18em] uppercase">
                            {t("pages.agent.skills.path")}
                          </div>
                          <div className="bg-muted text-foreground overflow-x-auto rounded-lg px-3 py-2 font-mono text-xs leading-relaxed">
                            {skill.path}
                          </div>
                        </CardContent>
                      </Card>
                    )
                  })}
                </div>
              ) : (
                <Card className="border-dashed">
                  <CardContent className="text-muted-foreground py-10 text-center text-sm">
                    {t("pages.agent.skills.empty")}
                  </CardContent>
                </Card>
              )}
            </section>
          )}
        </div>
      </div>

      <Sheet
        open={selectedSkill !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedSkill(null)
        }}
      >
        <SheetContent
          side="right"
          className="w-full gap-0 p-0 data-[side=right]:!w-full data-[side=right]:sm:!w-[560px] data-[side=right]:sm:!max-w-[560px]"
        >
          <SheetHeader className="border-b px-6 py-5">
            <SheetTitle>
              {selectedSkill
                ? skillLabels(selectedSkill).title
                : t("pages.agent.skills.viewer_title")}
            </SheetTitle>
            <SheetDescription>
              {selectedSkill
                ? skillLabels(selectedSkill).description
                : t("pages.agent.skills.viewer_description")}
            </SheetDescription>
          </SheetHeader>

          <div className="flex-1 overflow-auto px-6 py-5">
            {isSkillDetailLoading ? (
              <div className="text-muted-foreground text-sm">
                {t("pages.agent.skills.loading_detail")}
              </div>
            ) : skillDetailError ? (
              <div className="text-destructive text-sm">
                {t("pages.agent.skills.load_detail_error")}
              </div>
            ) : selectedSkillDetail ? (
              <div className="space-y-5">
                <div className="prose prose-sm dark:prose-invert prose-pre:rounded-lg prose-pre:border prose-pre:bg-zinc-950 prose-pre:p-3 max-w-none">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    rehypePlugins={[rehypeRaw, rehypeSanitize]}
                  >
                    {selectedSkillDetail.content}
                  </ReactMarkdown>
                </div>
              </div>
            ) : null}
          </div>
        </SheetContent>
      </Sheet>

      <AlertDialog
        open={skillPendingDelete !== null}
        onOpenChange={(open) => {
          if (!open) setSkillPendingDelete(null)
        }}
      >
        <AlertDialogContent size="sm">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("pages.agent.skills.delete_title")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("pages.agent.skills.delete_description", {
                name: skillPendingDelete
                  ? skillLabels(skillPendingDelete).title
                  : "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {t("common.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteMutation.isPending || !skillPendingDelete}
              onClick={() => {
                if (skillPendingDelete)
                  deleteMutation.mutate(skillPendingDelete.name)
              }}
            >
              {deleteMutation.isPending ? (
                <IconLoader2 className="size-4 animate-spin" />
              ) : (
                <IconTrash className="size-4" />
              )}
              {t("pages.agent.skills.delete_confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {uploadModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div className="bg-background w-full max-w-xl rounded-2xl border shadow-2xl">
            <div className="flex items-center justify-between border-b px-5 py-3.5">
              <div className="text-xl font-semibold tracking-tight">上传技能</div>
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground hover:text-foreground"
                onClick={() => {
                  setUploadModalOpen(false)
                  setDocWarningsExpanded(false)
                  setImportFeedback(null)
                }}
                disabled={importMutation.isPending}
              >
                <IconX className="size-4" />
              </Button>
            </div>

            <div className="space-y-4 px-5 py-4">
              <div
                className={`flex h-36 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed text-center transition-colors ${
                  dropActive
                    ? "border-primary bg-primary/5 shadow-sm"
                    : "border-muted-foreground/25 bg-muted/20 hover:bg-muted/30"
                }`}
                onClick={handleDropZoneClick}
                onDragOver={handleDropZoneDragOver}
                onDragLeave={handleDropZoneDragLeave}
                onDrop={handleDropZoneDrop}
              >
                {importMutation.isPending ? (
                  <IconLoader2 className="text-muted-foreground mb-2.5 size-7 animate-spin" />
                ) : (
                  <IconUpload className="text-muted-foreground mb-2.5 size-7" />
                )}
                <div className="text-foreground text-xl font-medium leading-none">
                  拖拽文件或点击上传
                </div>
                <div className="text-muted-foreground mt-2 text-sm">
                  支持 `SKILL.md`、`.zip`，或使用“上传文件夹”
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  onClick={handleImportClick}
                  disabled={importMutation.isPending}
                >
                  上传文件
                </Button>
                <Button
                  variant="outline"
                  onClick={handleFolderImportClick}
                  disabled={importMutation.isPending}
                >
                  上传文件夹
                </Button>
              </div>

              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 rounded border-muted-foreground/40"
                  checked={autoInstallLowRisk}
                  onChange={(e) => setAutoInstallLowRisk(e.target.checked)}
                  disabled={importMutation.isPending}
                />
                非高风险自动安装
              </label>

              <div className="space-y-2 rounded-lg border bg-muted/20 px-4 py-3">
                <div className="text-sm font-semibold">文件要求</div>
                <ul className="text-muted-foreground list-disc space-y-1 pl-5 text-sm leading-6">
                  <li>文件夹或 `.zip` 必须包含 `SKILL.md` 文件</li>
                  <li>`SKILL.md` 需包含 YAML frontmatter 的 `name` 和 `description`</li>
                </ul>
              </div>

              {importFeedback ? (
                <div
                  className={`space-y-3 rounded-xl border px-3.5 py-3.5 ${
                    importFeedback.warnings.length > 0
                      ? "border-amber-300/70 bg-amber-50/80 dark:border-amber-700/60 dark:bg-amber-950/20"
                      : "border-emerald-300/70 bg-emerald-50/80 dark:border-emerald-700/60 dark:bg-emerald-950/20"
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <div
                      className={`mt-0.5 rounded-full p-1.5 ${
                        importFeedback.warnings.length > 0
                          ? "bg-amber-100 text-amber-700 dark:bg-amber-900/60 dark:text-amber-300"
                          : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/60 dark:text-emerald-300"
                      }`}
                    >
                      {importFeedback.warnings.length > 0 ? (
                        <IconAlertTriangle className="size-4" />
                      ) : (
                        <IconCheck className="size-4" />
                      )}
                    </div>
                    <div className="space-y-1">
                      <div className="text-sm font-semibold">
                        {importFeedback.reviewWarnings.length > 0
                          ? "技能已导入，建议重点复核"
                          : importFeedback.documentationWarnings.length > 0
                            ? "技能已导入，包含文档示例提示"
                            : "技能已导入，可以直接使用"}
                      </div>
                      <div className="text-muted-foreground text-sm leading-6">
                        {importFeedback.name ? (
                          <>已导入技能：{importFeedback.name}</>
                        ) : (
                          <>技能包已通过基础校验并完成导入。</>
                        )}
                      </div>
                    </div>
                  </div>

                  {importFeedback.warnings.length > 0 ? (
                    <div className="space-y-3">
                      {importFeedback.reviewWarnings.length > 0 ? (
                        <div className="space-y-2">
                          <div className="text-sm font-medium">重点复核项</div>
                          <ul className="max-h-40 space-y-2 overflow-y-auto pr-1 text-sm leading-6">
                            {importFeedback.reviewWarnings.map((warning) => (
                              <li
                                key={`review-${warning}`}
                                className="rounded-lg border border-amber-200/70 bg-background/70 px-3 py-2 dark:border-amber-800/60"
                              >
                                {warning}
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : null}

                      {importFeedback.documentationWarnings.length > 0 ? (
                        <div className="space-y-2">
                          <button
                            type="button"
                            className="flex w-full items-center justify-between rounded-lg border border-amber-200/60 bg-background/60 px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-background dark:border-amber-800/50"
                            onClick={() =>
                              setDocWarningsExpanded((expanded) => !expanded)
                            }
                          >
                            <span>
                              文档示例类提示
                              {` (${importFeedback.documentationWarnings.length})`}
                            </span>
                            {docWarningsExpanded ? (
                              <IconChevronUp className="size-4" />
                            ) : (
                              <IconChevronDown className="size-4" />
                            )}
                          </button>
                          {docWarningsExpanded ? (
                            <ul className="max-h-32 space-y-2 overflow-y-auto pr-1 text-sm leading-6">
                              {importFeedback.documentationWarnings.map((warning) => (
                                <li
                                  key={`doc-${warning}`}
                                  className="rounded-lg border border-amber-200/60 bg-background/60 px-3 py-2 text-muted-foreground dark:border-amber-800/50"
                                >
                                  {warning}
                                </li>
                              ))}
                            </ul>
                          ) : (
                            <div className="px-1 text-xs leading-5 text-muted-foreground">
                              这类提示多来自 README 等说明文档中的命令示例，一般不代表技能会自动执行这些操作。
                            </div>
                          )}
                        </div>
                      ) : null}
                    </div>
                  ) : (
                    <div className="rounded-lg border border-emerald-200/70 bg-background/70 px-3 py-2 text-sm leading-6 dark:border-emerald-800/60">
                      未发现需要额外提示的内容。若技能包含外部下载、系统命令或凭据操作，仍建议上线前人工复核。
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
