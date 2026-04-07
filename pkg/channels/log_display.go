package channels

// LogDisplayName returns the channel name shown in console logs (TsingPaw branding).
// Config keys, JSON, and internal routing identifiers (e.g. "pico") stay unchanged.
func LogDisplayName(id string) string {
	switch id {
	case "pico":
		return "tsingpaw"
	case "pico_client":
		return "tsingpaw_client"
	default:
		return id
	}
}

// LogDisplayNames applies LogDisplayName to each id in order.
func LogDisplayNames(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = LogDisplayName(id)
	}
	return out
}
