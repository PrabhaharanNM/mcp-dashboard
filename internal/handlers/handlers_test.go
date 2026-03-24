package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/database"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/mttr"
)

// testDB creates a temporary SQLite database for handler tests.
func testDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return db
}

// --- Ingest Jenkins Tests ---

func TestIngestJenkins_Success(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	payload := models.IngestJenkinsPayload{
		AnalysisID:       "j-001",
		JobName:          "my-pipeline",
		BuildNumber:      42,
		BuildURL:         "http://jenkins/job/my-pipeline/42",
		Repository:       "myorg/myrepo",
		Branch:           "main",
		CommitHash:       "abc123",
		FailedStage:      "Build - AP",
		Status:           "failure",
		Category:         "CodeChange",
		RootCauseSummary: "Compilation error in Service.java",
		ResponsibleTeam:  "Backend",
		Confidence:       "high",
		Evidence:         []string{"error in Service.java:42"},
		NextSteps:        []string{"Fix the compilation error"},
		ErrorMessages:    []string{"cannot find symbol"},
		AnalysisTimeMs:   3200,
		JiraTicketKey:    "PROJ-123",
		JiraTicketUrl:    "https://jira.example.com/browse/PROJ-123",
		Developer:        "dev1",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.IngestJenkins(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["analysisId"] != "j-001" {
		t.Errorf("expected analysisId j-001, got %q", resp["analysisId"])
	}

	// Verify data persisted
	failures, _ := db.ListFailures(10, 0, "", "", "")
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if failures[0].Category != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", failures[0].Category)
	}
	if failures[0].Developer != "dev1" {
		t.Errorf("expected developer dev1, got %q", failures[0].Developer)
	}

	// Verify Jira ticket created
	tickets, _ := db.ListPendingJiraTickets()
	if len(tickets) != 1 {
		t.Fatalf("expected 1 jira ticket, got %d", len(tickets))
	}
	if tickets[0].TicketKey != "PROJ-123" {
		t.Errorf("expected PROJ-123, got %q", tickets[0].TicketKey)
	}

	// Verify pipeline created
	pipelines, _ := db.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].Platform != "jenkins" {
		t.Errorf("expected jenkins platform, got %q", pipelines[0].Platform)
	}
}

func TestIngestJenkins_InvalidJSON(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.IngestJenkins(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIngestJenkins_MethodNotAllowed(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	req := httptest.NewRequest(http.MethodGet, "/api/ingest/jenkins", nil)
	w := httptest.NewRecorder()

	h.IngestJenkins(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestIngestJenkins_EmptyJobName(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	payload := models.IngestJenkinsPayload{
		AnalysisID: "j-empty",
		Category:   "Unknown",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.IngestJenkins(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	pipelines, _ := db.ListPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].PipelineName != "unknown" {
		t.Errorf("expected 'unknown' pipeline name, got %q", pipelines[0].PipelineName)
	}
}

// --- Ingest GitHub Tests ---

func TestIngestGithub_Success(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	payload := models.IngestGithubPayload{
		AnalysisID:       "gh-001",
		Owner:            "myorg",
		Repo:             "myrepo",
		Workflow:         "CI",
		RunID:            98765,
		RunNumber:        42,
		Actor:            "dev2",
		SHA:              "def456",
		Ref:              "refs/heads/main",
		FailedJob:        "build",
		FailedStep:       "Run tests",
		Status:           "failure",
		Category:         "TestFailure",
		RootCauseSummary: "3 tests failed in auth_test.go",
		ResponsibleTeam:  "Auth",
		Confidence:       "high",
		Evidence:         []string{"TestLogin failed", "TestLogout failed"},
		ErrorMessages:    []string{"FAIL: TestLogin"},
		AnalysisTimeMs:   2800,
		GithubIssueUrl:   "https://github.com/myorg/myrepo/issues/42",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/github", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.IngestGithub(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	failures, _ := db.ListFailures(10, 0, "", "", "")
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if failures[0].Platform != "github" {
		t.Errorf("expected github, got %q", failures[0].Platform)
	}
	if failures[0].Developer != "dev2" {
		t.Errorf("expected dev2, got %q", failures[0].Developer)
	}
}

func TestIngestGithub_WithJiraTicket(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	payload := models.IngestGithubPayload{
		AnalysisID:       "gh-jira",
		Owner:            "org",
		Repo:             "repo",
		Workflow:         "CI",
		RunID:            100,
		RunNumber:        1,
		Category:         "Infrastructure",
		RootCauseSummary: "K8s OOM",
		JiraTicketKey:    "OPS-456",
		JiraTicketUrl:    "https://jira.example.com/browse/OPS-456",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/github", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.IngestGithub(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	tickets, _ := db.ListPendingJiraTickets()
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(tickets))
	}
	if tickets[0].TicketKey != "OPS-456" {
		t.Errorf("expected OPS-456, got %q", tickets[0].TicketKey)
	}
}

func TestIngestGithub_InvalidJSON(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	req := httptest.NewRequest(http.MethodPost, "/api/ingest/github", bytes.NewReader([]byte("{bad")))
	w := httptest.NewRecorder()
	h.IngestGithub(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Resolve Failure Tests ---

func TestResolveFailure_Success(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	// First ingest a failure
	payload := models.IngestJenkinsPayload{
		AnalysisID: "resolve-test",
		JobName:    "test-job",
		Category:   "CodeChange",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.IngestJenkins(w, req)

	// Now resolve it
	req = httptest.NewRequest(http.MethodPost, "/api/failures/resolve-test/resolve", nil)
	w = httptest.NewRecorder()
	h.ResolveFailure(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveFailure_MissingID(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	req := httptest.NewRequest(http.MethodPost, "/api/failures//resolve", nil)
	w := httptest.NewRecorder()
	h.ResolveFailure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestResolveFailure_MethodNotAllowed(t *testing.T) {
	db := testDB(t)
	h := NewIngestHandler(db)

	req := httptest.NewRequest(http.MethodGet, "/api/failures/x/resolve", nil)
	w := httptest.NewRecorder()
	h.ResolveFailure(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// --- API Handler Tests ---

func TestAPIDashboard_Empty(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	h := NewAPIHandler(db, mc)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	w := httptest.NewRecorder()
	h.Dashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var data models.DashboardData
	json.Unmarshal(w.Body.Bytes(), &data)

	if data.TotalFailures != 0 {
		t.Errorf("expected 0 failures, got %d", data.TotalFailures)
	}
	if data.Pipelines == nil {
		t.Error("pipelines should be empty array, not null")
	}
	if data.Failures == nil {
		t.Error("failures should be empty array, not null")
	}
	if data.PendingJira == nil {
		t.Error("pending jira should be empty array, not null")
	}
}

func TestAPIDashboard_WithData(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	// Ingest 2 Jenkins + 1 GitHub failure
	for _, p := range []models.IngestJenkinsPayload{
		{AnalysisID: "j1", JobName: "pipeline-a", Category: "CodeChange", ResponsibleTeam: "Backend", Developer: "dev1"},
		{AnalysisID: "j2", JobName: "pipeline-b", Category: "Infrastructure", ResponsibleTeam: "DevOps", Developer: "dev2"},
	} {
		body, _ := json.Marshal(p)
		req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
		w := httptest.NewRecorder()
		ih.IngestJenkins(w, req)
	}

	ghPayload := models.IngestGithubPayload{
		AnalysisID: "gh1", Owner: "org", Repo: "repo", Workflow: "CI",
		RunID: 1, RunNumber: 1, Category: "TestFailure", ResponsibleTeam: "QA",
	}
	body, _ := json.Marshal(ghPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/github", bytes.NewReader(body))
	w := httptest.NewRecorder()
	ih.IngestGithub(w, req)

	// Get dashboard
	req = httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	w = httptest.NewRecorder()
	ah.Dashboard(w, req)

	var data models.DashboardData
	json.Unmarshal(w.Body.Bytes(), &data)

	if data.TotalFailures != 3 {
		t.Errorf("expected 3 failures, got %d", data.TotalFailures)
	}
	if data.JenkinsCount != 2 {
		t.Errorf("expected 2 jenkins, got %d", data.JenkinsCount)
	}
	if data.GithubCount != 1 {
		t.Errorf("expected 1 github, got %d", data.GithubCount)
	}
	if len(data.Pipelines) != 3 {
		t.Errorf("expected 3 pipelines, got %d", len(data.Pipelines))
	}
}

func TestAPIFailures_WithFilters(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	// Ingest various failures
	for _, p := range []models.IngestJenkinsPayload{
		{AnalysisID: "f1", JobName: "j1", Category: "CodeChange", ResponsibleTeam: "Backend"},
		{AnalysisID: "f2", JobName: "j2", Category: "Infrastructure", ResponsibleTeam: "DevOps"},
		{AnalysisID: "f3", JobName: "j3", Category: "CodeChange", ResponsibleTeam: "Frontend"},
	} {
		body, _ := json.Marshal(p)
		req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
		w := httptest.NewRecorder()
		ih.IngestJenkins(w, req)
	}

	// Filter by category
	req := httptest.NewRequest(http.MethodGet, "/api/failures?category=CodeChange", nil)
	w := httptest.NewRecorder()
	ah.Failures(w, req)

	var failures []models.Failure
	json.Unmarshal(w.Body.Bytes(), &failures)
	if len(failures) != 2 {
		t.Errorf("expected 2 CodeChange failures, got %d", len(failures))
	}

	// Filter by team
	req = httptest.NewRequest(http.MethodGet, "/api/failures?team=DevOps", nil)
	w = httptest.NewRecorder()
	ah.Failures(w, req)

	json.Unmarshal(w.Body.Bytes(), &failures)
	if len(failures) != 1 {
		t.Errorf("expected 1 DevOps failure, got %d", len(failures))
	}

	// Filter by platform
	req = httptest.NewRequest(http.MethodGet, "/api/failures?platform=jenkins", nil)
	w = httptest.NewRecorder()
	ah.Failures(w, req)

	json.Unmarshal(w.Body.Bytes(), &failures)
	if len(failures) != 3 {
		t.Errorf("expected 3 jenkins failures, got %d", len(failures))
	}
}

func TestAPIFailures_Pagination(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	for i := 0; i < 10; i++ {
		p := models.IngestJenkinsPayload{
			AnalysisID: "page-" + string(rune('a'+i)),
			JobName:    "job",
			Category:   "Unknown",
		}
		body, _ := json.Marshal(p)
		req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
		w := httptest.NewRecorder()
		ih.IngestJenkins(w, req)
	}

	// Request with limit=3
	req := httptest.NewRequest(http.MethodGet, "/api/failures?limit=3", nil)
	w := httptest.NewRecorder()
	ah.Failures(w, req)

	var failures []models.Failure
	json.Unmarshal(w.Body.Bytes(), &failures)
	if len(failures) != 3 {
		t.Errorf("expected 3 failures with limit, got %d", len(failures))
	}
}

func TestAPIPipelines(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	payload := models.IngestJenkinsPayload{AnalysisID: "pipe-test", JobName: "my-pipe", Category: "Unknown"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()
	ih.IngestJenkins(w, req)

	req = httptest.NewRequest(http.MethodGet, "/api/pipelines", nil)
	w = httptest.NewRecorder()
	ah.Pipelines(w, req)

	var pipelines []models.Pipeline
	json.Unmarshal(w.Body.Bytes(), &pipelines)
	if len(pipelines) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].PipelineName != "my-pipe" {
		t.Errorf("expected my-pipe, got %q", pipelines[0].PipelineName)
	}
}

func TestAPIPendingJira(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	payload := models.IngestJenkinsPayload{
		AnalysisID:    "jira-test",
		JobName:       "j",
		JiraTicketKey: "TEST-1",
		JiraTicketUrl: "https://jira/TEST-1",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()
	ih.IngestJenkins(w, req)

	req = httptest.NewRequest(http.MethodGet, "/api/jira/pending", nil)
	w = httptest.NewRecorder()
	ah.PendingJira(w, req)

	var tickets []models.JiraTicket
	json.Unmarshal(w.Body.Bytes(), &tickets)
	if len(tickets) != 1 {
		t.Errorf("expected 1 pending ticket, got %d", len(tickets))
	}
}

func TestAPIMTTR_Empty(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ah := NewAPIHandler(db, mc)

	req := httptest.NewRequest(http.MethodGet, "/api/mttr", nil)
	w := httptest.NewRecorder()
	ah.MTTR(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats models.MTTRStats
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.TotalResolved != 0 {
		t.Errorf("expected 0 resolved, got %d", stats.TotalResolved)
	}
}

// --- Full End-to-End Flow ---

func TestE2E_IngestResolveAndDashboard(t *testing.T) {
	db := testDB(t)
	mc := mttr.New(db.Conn())
	ih := NewIngestHandler(db)
	ah := NewAPIHandler(db, mc)

	// 1. Ingest Jenkins failure
	jp := models.IngestJenkinsPayload{
		AnalysisID:      "e2e-j1",
		JobName:         "e2e-pipeline",
		BuildNumber:     1,
		Category:        "CodeChange",
		ResponsibleTeam: "Backend",
		Developer:       "dev1",
		JiraTicketKey:   "E2E-1",
		JiraTicketUrl:   "https://jira/E2E-1",
	}
	body, _ := json.Marshal(jp)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest/jenkins", bytes.NewReader(body))
	w := httptest.NewRecorder()
	ih.IngestJenkins(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest jenkins failed: %d", w.Code)
	}

	// 2. Ingest GitHub failure
	gp := models.IngestGithubPayload{
		AnalysisID:      "e2e-gh1",
		Owner:           "org",
		Repo:            "repo",
		Workflow:        "CI",
		RunID:           100,
		RunNumber:       1,
		Category:        "Infrastructure",
		ResponsibleTeam: "DevOps",
		Actor:           "dev2",
	}
	body, _ = json.Marshal(gp)
	req = httptest.NewRequest(http.MethodPost, "/api/ingest/github", bytes.NewReader(body))
	w = httptest.NewRecorder()
	ih.IngestGithub(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest github failed: %d", w.Code)
	}

	// 3. Resolve the Jenkins failure
	req = httptest.NewRequest(http.MethodPost, "/api/failures/e2e-j1/resolve", nil)
	w = httptest.NewRecorder()
	ih.ResolveFailure(w, req)
	if w.Code != 200 {
		t.Fatalf("resolve failed: %d %s", w.Code, w.Body.String())
	}

	// 4. Check dashboard
	req = httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	w = httptest.NewRecorder()
	ah.Dashboard(w, req)

	var dashboard models.DashboardData
	json.Unmarshal(w.Body.Bytes(), &dashboard)

	if dashboard.TotalFailures != 2 {
		t.Errorf("expected 2 total failures, got %d", dashboard.TotalFailures)
	}
	if dashboard.JenkinsCount != 1 {
		t.Errorf("expected 1 jenkins, got %d", dashboard.JenkinsCount)
	}
	if dashboard.GithubCount != 1 {
		t.Errorf("expected 1 github, got %d", dashboard.GithubCount)
	}

	// 5. Check MTTR has 1 resolved
	req = httptest.NewRequest(http.MethodGet, "/api/mttr", nil)
	w = httptest.NewRecorder()
	ah.MTTR(w, req)

	var stats models.MTTRStats
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.TotalResolved != 1 {
		t.Errorf("expected 1 resolved, got %d", stats.TotalResolved)
	}
	if stats.TotalUnresolved != 1 {
		t.Errorf("expected 1 unresolved, got %d", stats.TotalUnresolved)
	}
}
