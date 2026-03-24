package models

import "time"

const (
	PlatformJenkins = "jenkins"
	PlatformGithub  = "github"
)

// Pipeline represents a monitored CI/CD pipeline or workflow.
type Pipeline struct {
	ID           int64      `json:"id"`
	Platform     string     `json:"platform"`
	PipelineName string     `json:"pipelineName"`
	Repository   string     `json:"repository"`
	LastStatus   string     `json:"lastStatus"`
	LastBuildAt  *time.Time `json:"lastBuildAt"`
}

// Failure represents a single analyzed build failure from either platform.
type Failure struct {
	ID              int64  `json:"id"`
	AnalysisID      string `json:"analysisId"`
	Platform        string `json:"platform"`
	PipelineID      int64  `json:"pipelineId"`
	BuildIdentifier string `json:"buildIdentifier"`
	BuildURL        string `json:"buildUrl"`

	// Jenkins fields
	JobName     string `json:"jobName"`
	BuildNumber int    `json:"buildNumber"`
	Branch      string `json:"branch"`
	CommitHash  string `json:"commitHash"`
	FailedStage string `json:"failedStage"`

	// GitHub fields
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	Workflow   string `json:"workflow"`
	RunID      int64  `json:"runId"`
	RunNumber  int    `json:"runNumber"`
	Actor      string `json:"actor"`
	SHA        string `json:"sha"`
	Ref        string `json:"ref"`
	FailedStep string `json:"failedStep"`
	FailedJob  string `json:"failedJob"`

	// Analysis results
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

	// Integrations
	JiraTicketKey  string `json:"jiraTicketKey"`
	JiraTicketURL  string `json:"jiraTicketUrl"`
	GithubIssueURL string `json:"githubIssueUrl"`

	// MTTR
	FailedAt    time.Time  `json:"failedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
	MTTRSeconds *int64     `json:"mttrSeconds"`

	// Developer
	Developer string `json:"developer"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// JiraTicket tracks the status of a Jira ticket created by the system.
type JiraTicket struct {
	ID        int64     `json:"id"`
	FailureID int64     `json:"failureId"`
	TicketKey string    `json:"ticketKey"`
	TicketURL string    `json:"ticketUrl"`
	Summary   string    `json:"summary"`
	Status    string    `json:"status"`
	Assignee  string    `json:"assignee"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// IngestJenkinsPayload is POSTed by the Jenkins plugin.
type IngestJenkinsPayload struct {
	AnalysisID       string   `json:"analysisId"`
	JobName          string   `json:"jobName"`
	BuildNumber      int      `json:"buildNumber"`
	BuildURL         string   `json:"buildUrl"`
	Repository       string   `json:"repository"`
	Branch           string   `json:"branch"`
	CommitHash       string   `json:"commitHash"`
	FailedStage      string   `json:"failedStage"`
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
	JiraTicketUrl    string   `json:"jiraTicketUrl"`
	Developer        string   `json:"developer"`
}

// IngestGithubPayload is POSTed by the GitHub Action.
type IngestGithubPayload struct {
	AnalysisID       string   `json:"analysisId"`
	Owner            string   `json:"owner"`
	Repo             string   `json:"repo"`
	Workflow         string   `json:"workflow"`
	RunID            int64    `json:"runId"`
	RunNumber        int      `json:"runNumber"`
	Actor            string   `json:"actor"`
	SHA              string   `json:"sha"`
	Ref              string   `json:"ref"`
	FailedStep       string   `json:"failedStep"`
	FailedJob        string   `json:"failedJob"`
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
	JiraTicketUrl    string   `json:"jiraTicketUrl"`
	GithubIssueUrl   string   `json:"githubIssueUrl"`
}

// MTTRStats holds computed MTTR statistics.
type MTTRStats struct {
	OverallAvgSeconds float64            `json:"overallAvgSeconds"`
	Avg7DaySeconds    float64            `json:"avg7DaySeconds"`
	Avg30DaySeconds   float64            `json:"avg30DaySeconds"`
	ByTeam            map[string]float64 `json:"byTeam"`
	ByCategory        map[string]float64 `json:"byCategory"`
	TotalResolved     int                `json:"totalResolved"`
	TotalUnresolved   int                `json:"totalUnresolved"`
}

// DashboardData is the complete view-model for the dashboard.
type DashboardData struct {
	Pipelines        []Pipeline     `json:"pipelines"`
	Failures         []Failure      `json:"failures"`
	TeamDistribution map[string]int `json:"teamDistribution"`
	CategoryDist     map[string]int `json:"categoryDistribution"`
	MTTRStats        MTTRStats      `json:"mttrStats"`
	PendingJira      []JiraTicket   `json:"pendingJira"`
	TotalFailures    int            `json:"totalFailures"`
	JenkinsCount     int            `json:"jenkinsCount"`
	GithubCount      int            `json:"githubCount"`
}
