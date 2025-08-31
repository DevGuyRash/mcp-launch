package server

// RenderOptions returns server options without numeric shortcuts.
// Numeric shortcuts 1–4 intentionally removed per UX guidelines.
func RenderOptions() []string {
    return []string{
        "Open Tunnel",
        "OpenAPI Summary",
        "Launch Stack",
        "Stop Stack",
    }
}

