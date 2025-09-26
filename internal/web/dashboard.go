package web

import (
	"net/http"
	"path/filepath"
)

// ServeDashboard serves the dashboard HTML file
func ServeDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Serve the dashboard HTML file
	dashboardPath := filepath.Join("web", "dashboard.html")
	http.ServeFile(w, r, dashboardPath)
}
