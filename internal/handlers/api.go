package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/database"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/mttr"
)

// APIHandler serves JSON API endpoints.
type APIHandler struct {
	db   *database.DB
	mttr *mttr.Calculator
}

func NewAPIHandler(db *database.DB, mc *mttr.Calculator) *APIHandler {
	return &APIHandler{db: db, mttr: mc}
}

// Dashboard handles GET /api/dashboard — full dashboard data.
func (h *APIHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	pipelines, _ := h.db.ListPipelines()
	failures, _ := h.db.ListFailures(100, 0, "", "", "")
	teamDist, _ := h.db.TeamDistribution()
	catDist, _ := h.db.CategoryDistribution()
	mttrStats, _ := h.mttr.Calculate()
	pendingJira, _ := h.db.ListPendingJiraTickets()
	total, _ := h.db.TotalFailures()
	jenkins, github, _ := h.db.CountByPlatform()

	if pipelines == nil {
		pipelines = []models.Pipeline{}
	}
	if failures == nil {
		failures = []models.Failure{}
	}
	if pendingJira == nil {
		pendingJira = []models.JiraTicket{}
	}

	data := models.DashboardData{
		Pipelines:        pipelines,
		Failures:         failures,
		TeamDistribution: teamDist,
		CategoryDist:     catDist,
		MTTRStats:        *mttrStats,
		PendingJira:      pendingJira,
		TotalFailures:    total,
		JenkinsCount:     jenkins,
		GithubCount:      github,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Failures handles GET /api/failures with optional filters.
func (h *APIHandler) Failures(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	failures, err := h.db.ListFailures(limit, offset, q.Get("platform"), q.Get("team"), q.Get("category"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if failures == nil {
		failures = []models.Failure{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(failures)
}

// MTTR handles GET /api/mttr — live MTTR statistics.
func (h *APIHandler) MTTR(w http.ResponseWriter, r *http.Request) {
	stats, err := h.mttr.Calculate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Pipelines handles GET /api/pipelines.
func (h *APIHandler) Pipelines(w http.ResponseWriter, r *http.Request) {
	pipelines, err := h.db.ListPipelines()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pipelines == nil {
		pipelines = []models.Pipeline{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pipelines)
}

// PendingJira handles GET /api/jira/pending.
func (h *APIHandler) PendingJira(w http.ResponseWriter, r *http.Request) {
	tickets, err := h.db.ListPendingJiraTickets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tickets == nil {
		tickets = []models.JiraTicket{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickets)
}
