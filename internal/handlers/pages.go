package handlers

import (
	"embed"
	"html/template"
	"net/http"
)

// PageHandler serves the dashboard HTML page.
type PageHandler struct {
	tmpl *template.Template
}

func NewPageHandler(fs embed.FS) *PageHandler {
	tmpl := template.Must(template.ParseFS(fs, "templates/dashboard.html"))
	return &PageHandler{tmpl: tmpl}
}

// Dashboard handles GET / — serves the main dashboard page.
func (h *PageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.Execute(w, nil)
}
