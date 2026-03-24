package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/database"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
)

// IngestHandler handles POST requests to ingest analysis results.
type IngestHandler struct {
	db *database.DB
}

func NewIngestHandler(db *database.DB) *IngestHandler {
	return &IngestHandler{db: db}
}

// IngestJenkins handles POST /api/ingest/jenkins
func (h *IngestHandler) IngestJenkins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p models.IngestJenkinsPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	pipelineName := p.JobName
	if pipelineName == "" {
		pipelineName = "unknown"
	}

	pipelineID, err := h.db.UpsertPipeline(&models.Pipeline{
		Platform:     models.PlatformJenkins,
		PipelineName: pipelineName,
		Repository:   p.Repository,
		LastStatus:   "failure",
		LastBuildAt:  &now,
	})
	if err != nil {
		log.Printf("upsert pipeline: %v", err)
	}

	developer := p.Developer
	if developer == "" {
		developer = "unknown"
	}

	f := &models.Failure{
		AnalysisID:       p.AnalysisID,
		Platform:         models.PlatformJenkins,
		PipelineID:       pipelineID,
		BuildIdentifier:  fmt.Sprintf("%s #%d", p.JobName, p.BuildNumber),
		BuildURL:         p.BuildURL,
		JobName:          p.JobName,
		BuildNumber:      p.BuildNumber,
		Branch:           p.Branch,
		CommitHash:       p.CommitHash,
		FailedStage:      p.FailedStage,
		Status:           p.Status,
		Category:         p.Category,
		RootCauseSummary: p.RootCauseSummary,
		RootCauseDetails: p.RootCauseDetails,
		ResponsibleTeam:  p.ResponsibleTeam,
		TeamEmail:        p.TeamEmail,
		Confidence:       p.Confidence,
		Evidence:         p.Evidence,
		NextSteps:        p.NextSteps,
		ErrorMessages:    p.ErrorMessages,
		AnalysisTimeMs:   p.AnalysisTimeMs,
		JiraTicketKey:    p.JiraTicketKey,
		JiraTicketURL:    p.JiraTicketUrl,
		FailedAt:         now,
		Developer:        developer,
	}

	if err := h.db.InsertFailure(f); err != nil {
		log.Printf("insert failure: %v", err)
		http.Error(w, "failed to store failure", http.StatusInternalServerError)
		return
	}

	// If Jira ticket was created, track it
	if p.JiraTicketKey != "" {
		failureID, _ := h.db.GetFailureIDByAnalysis(p.AnalysisID)
		h.db.UpsertJiraTicket(&models.JiraTicket{
			FailureID: failureID,
			TicketKey: p.JiraTicketKey,
			TicketURL: p.JiraTicketUrl,
			Summary:   p.RootCauseSummary,
			Status:    "Open",
			Assignee:  p.ResponsibleTeam,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "analysisId": p.AnalysisID})
}

// IngestGithub handles POST /api/ingest/github
func (h *IngestHandler) IngestGithub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p models.IngestGithubPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	pipelineName := p.Workflow
	if pipelineName == "" {
		pipelineName = fmt.Sprintf("%s/%s", p.Owner, p.Repo)
	}

	pipelineID, err := h.db.UpsertPipeline(&models.Pipeline{
		Platform:     models.PlatformGithub,
		PipelineName: pipelineName,
		Repository:   fmt.Sprintf("%s/%s", p.Owner, p.Repo),
		LastStatus:   "failure",
		LastBuildAt:  &now,
	})
	if err != nil {
		log.Printf("upsert pipeline: %v", err)
	}

	developer := p.Actor
	if developer == "" {
		developer = "unknown"
	}

	f := &models.Failure{
		AnalysisID:       p.AnalysisID,
		Platform:         models.PlatformGithub,
		PipelineID:       pipelineID,
		BuildIdentifier:  fmt.Sprintf("%s/%s #%d", p.Owner, p.Repo, p.RunNumber),
		BuildURL:         fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", p.Owner, p.Repo, p.RunID),
		Owner:            p.Owner,
		Repo:             p.Repo,
		Workflow:         p.Workflow,
		RunID:            p.RunID,
		RunNumber:        p.RunNumber,
		Actor:            p.Actor,
		SHA:              p.SHA,
		Ref:              p.Ref,
		FailedStep:       p.FailedStep,
		FailedJob:        p.FailedJob,
		Status:           p.Status,
		Category:         p.Category,
		RootCauseSummary: p.RootCauseSummary,
		RootCauseDetails: p.RootCauseDetails,
		ResponsibleTeam:  p.ResponsibleTeam,
		TeamEmail:        p.TeamEmail,
		Confidence:       p.Confidence,
		Evidence:         p.Evidence,
		NextSteps:        p.NextSteps,
		ErrorMessages:    p.ErrorMessages,
		AnalysisTimeMs:   p.AnalysisTimeMs,
		JiraTicketKey:    p.JiraTicketKey,
		JiraTicketURL:    p.JiraTicketUrl,
		GithubIssueURL:   p.GithubIssueUrl,
		FailedAt:         now,
		Developer:        developer,
	}

	if err := h.db.InsertFailure(f); err != nil {
		log.Printf("insert failure: %v", err)
		http.Error(w, "failed to store failure", http.StatusInternalServerError)
		return
	}

	if p.JiraTicketKey != "" {
		failureID, _ := h.db.GetFailureIDByAnalysis(p.AnalysisID)
		h.db.UpsertJiraTicket(&models.JiraTicket{
			FailureID: failureID,
			TicketKey: p.JiraTicketKey,
			TicketURL: p.JiraTicketUrl,
			Summary:   p.RootCauseSummary,
			Status:    "Open",
			Assignee:  p.ResponsibleTeam,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "analysisId": p.AnalysisID})
}

// ResolveFailure handles POST /api/failures/{id}/resolve
func (h *IngestHandler) ResolveFailure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract analysis ID from path: /api/failures/{id}/resolve
	path := strings.TrimPrefix(r.URL.Path, "/api/failures/")
	analysisID := strings.TrimSuffix(path, "/resolve")
	if analysisID == "" {
		http.Error(w, "missing analysis ID", http.StatusBadRequest)
		return
	}

	if err := h.db.ResolveFailure(analysisID, time.Now()); err != nil {
		http.Error(w, "failed to resolve: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved", "analysisId": analysisID})
}
