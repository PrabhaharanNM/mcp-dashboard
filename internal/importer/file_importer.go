package importer

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/database"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
)

// FileImporter polls a directory for JSON analysis results and ingests them.
type FileImporter struct {
	db      *database.DB
	dir     string
	seen    map[string]bool
	stopCh  chan struct{}
}

// New creates a FileImporter that watches the given directory.
func New(db *database.DB, dir string) *FileImporter {
	return &FileImporter{
		db:     db,
		dir:    dir,
		seen:   make(map[string]bool),
		stopCh: make(chan struct{}),
	}
}

// Start begins polling the directory every 10 seconds.
func (fi *FileImporter) Start() {
	go func() {
		fi.scan() // immediate first scan
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fi.scan()
			case <-fi.stopCh:
				return
			}
		}
	}()
}

// Stop stops the file importer.
func (fi *FileImporter) Stop() {
	close(fi.stopCh)
}

type fileResult struct {
	AnalysisID       string   `json:"analysisId"`
	Status           string   `json:"status"`
	Category         string   `json:"category"`
	RootCauseSummary string   `json:"rootCauseSummary"`
	RootCauseDetails string   `json:"rootCauseDetails"`
	ResponsibleTeam  string   `json:"responsibleTeam"`
	TeamEmail        string   `json:"teamEmail"`
	Confidence       string   `json:"confidence"`
	Evidence         []string `json:"evidence"`
	NextSteps        []string `json:"nextSteps"`
	ErrorMessages    []string `json:"errorMessages"`
	AnalysisTimeMs   int64    `json:"analysisTimeMs"`
	JiraTicketKey    string   `json:"jiraTicketKey"`
	JiraTicketURL    string   `json:"jiraTicketUrl"`
	GithubIssueURL   string   `json:"githubIssueUrl"`

	// snake_case fallbacks for Jenkins plugin output
	AnalysisIDSnake       string   `json:"analysis_id"`
	RootCauseSummarySnake string   `json:"root_cause_summary"`
	RootCauseDetailsSnake string   `json:"root_cause_details"`
	ResponsibleTeamSnake  string   `json:"responsible_team"`
	TeamEmailSnake        string   `json:"team_email"`
	ErrorMessagesSnake    []string `json:"error_messages"`
	NextStepsSnake        []string `json:"next_steps"`
	JiraTicketKeySnake    string   `json:"jira_ticket_key"`
}

func (fi *FileImporter) scan() {
	pattern := filepath.Join(fi.dir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		base := filepath.Base(file)
		if fi.seen[base] {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var result fileResult
		if err := json.Unmarshal(data, &result); err != nil {
			log.Printf("file importer: skipping %s: %v", base, err)
			fi.seen[base] = true
			continue
		}

		// Merge snake_case fallbacks
		analysisID := coalesce(result.AnalysisID, result.AnalysisIDSnake)
		if analysisID == "" {
			fi.seen[base] = true
			continue
		}

		f := &models.Failure{
			AnalysisID:       analysisID,
			Platform:         "unknown",
			BuildIdentifier:  base,
			Status:           result.Status,
			Category:         result.Category,
			RootCauseSummary: coalesce(result.RootCauseSummary, result.RootCauseSummarySnake),
			RootCauseDetails: coalesce(result.RootCauseDetails, result.RootCauseDetailsSnake),
			ResponsibleTeam:  coalesce(result.ResponsibleTeam, result.ResponsibleTeamSnake),
			TeamEmail:        coalesce(result.TeamEmail, result.TeamEmailSnake),
			Confidence:       result.Confidence,
			Evidence:         result.Evidence,
			NextSteps:        coalesceSlice(result.NextSteps, result.NextStepsSnake),
			ErrorMessages:    coalesceSlice(result.ErrorMessages, result.ErrorMessagesSnake),
			AnalysisTimeMs:   result.AnalysisTimeMs,
			JiraTicketKey:    coalesce(result.JiraTicketKey, result.JiraTicketKeySnake),
			JiraTicketURL:    result.JiraTicketURL,
			GithubIssueURL:   result.GithubIssueURL,
			FailedAt:         time.Now(),
		}

		if err := fi.db.InsertFailure(f); err != nil {
			log.Printf("file importer: insert %s: %v", base, err)
		} else {
			log.Printf("file importer: ingested %s (analysis: %s)", base, analysisID)
		}

		fi.seen[base] = true
	}
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func coalesceSlice(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}
