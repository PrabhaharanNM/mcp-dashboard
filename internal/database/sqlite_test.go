package database

import (
	"os"
	"testing"
	"time"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close(); os.Remove(path) })
	return db
}

func TestUpsertAndListPipelines(t *testing.T) {
	db := testDB(t)
	now := time.Now()

	id, err := db.UpsertPipeline(&models.Pipeline{
		Platform:     "jenkins",
		PipelineName: "build-job",
		Repository:   "org/repo",
		LastStatus:   "failure",
		LastBuildAt:  &now,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	pipelines, err := db.ListPipelines()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].PipelineName != "build-job" {
		t.Errorf("expected build-job, got %q", pipelines[0].PipelineName)
	}
}

func TestInsertAndListFailures(t *testing.T) {
	db := testDB(t)

	f := &models.Failure{
		AnalysisID:       "test-001",
		Platform:         "github",
		BuildIdentifier:  "org/repo #42",
		Category:         "CodeChange",
		RootCauseSummary: "Test failure in auth module",
		ResponsibleTeam:  "backend",
		Developer:        "dev1",
		FailedAt:         time.Now(),
		Evidence:         []string{"log line 1"},
		NextSteps:        []string{"fix test"},
		ErrorMessages:    []string{"assertion failed"},
	}
	if err := db.InsertFailure(f); err != nil {
		t.Fatalf("insert: %v", err)
	}

	failures, err := db.ListFailures(10, 0, "", "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if failures[0].Category != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", failures[0].Category)
	}
	if failures[0].Developer != "dev1" {
		t.Errorf("expected dev1, got %q", failures[0].Developer)
	}
}

func TestListFailuresWithFilters(t *testing.T) {
	db := testDB(t)

	for _, f := range []*models.Failure{
		{AnalysisID: "f1", Platform: "jenkins", Category: "CodeChange", ResponsibleTeam: "frontend", FailedAt: time.Now()},
		{AnalysisID: "f2", Platform: "github", Category: "Infrastructure", ResponsibleTeam: "backend", FailedAt: time.Now()},
		{AnalysisID: "f3", Platform: "github", Category: "CodeChange", ResponsibleTeam: "backend", FailedAt: time.Now()},
	} {
		db.InsertFailure(f)
	}

	// Filter by platform
	results, _ := db.ListFailures(10, 0, "github", "", "")
	if len(results) != 2 {
		t.Errorf("expected 2 github failures, got %d", len(results))
	}

	// Filter by team
	results, _ = db.ListFailures(10, 0, "", "backend", "")
	if len(results) != 2 {
		t.Errorf("expected 2 backend failures, got %d", len(results))
	}

	// Filter by category
	results, _ = db.ListFailures(10, 0, "", "", "CodeChange")
	if len(results) != 2 {
		t.Errorf("expected 2 CodeChange failures, got %d", len(results))
	}
}

func TestResolveFailure(t *testing.T) {
	db := testDB(t)

	db.InsertFailure(&models.Failure{
		AnalysisID: "resolve-001",
		Platform:   "jenkins",
		FailedAt:   time.Now().Add(-time.Hour),
	})

	if err := db.ResolveFailure("resolve-001", time.Now()); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	failures, _ := db.ListFailures(10, 0, "", "", "")
	if len(failures) != 1 {
		t.Fatal("expected 1 failure")
	}
	if failures[0].ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
	if failures[0].MTTRSeconds == nil {
		t.Error("expected mttr_seconds to be set")
	}
}

func TestTeamAndCategoryDistribution(t *testing.T) {
	db := testDB(t)

	for _, f := range []*models.Failure{
		{AnalysisID: "d1", Platform: "jenkins", Category: "CodeChange", ResponsibleTeam: "frontend", FailedAt: time.Now()},
		{AnalysisID: "d2", Platform: "github", Category: "Infrastructure", ResponsibleTeam: "backend", FailedAt: time.Now()},
		{AnalysisID: "d3", Platform: "github", Category: "CodeChange", ResponsibleTeam: "frontend", FailedAt: time.Now()},
	} {
		db.InsertFailure(f)
	}

	teamDist, err := db.TeamDistribution()
	if err != nil {
		t.Fatalf("team dist: %v", err)
	}
	if teamDist["frontend"] != 2 {
		t.Errorf("expected frontend=2, got %d", teamDist["frontend"])
	}
	if teamDist["backend"] != 1 {
		t.Errorf("expected backend=1, got %d", teamDist["backend"])
	}

	catDist, err := db.CategoryDistribution()
	if err != nil {
		t.Fatalf("cat dist: %v", err)
	}
	if catDist["CodeChange"] != 2 {
		t.Errorf("expected CodeChange=2, got %d", catDist["CodeChange"])
	}
}

func TestJiraTickets(t *testing.T) {
	db := testDB(t)

	db.InsertFailure(&models.Failure{AnalysisID: "jira-001", Platform: "jenkins", FailedAt: time.Now()})
	failureID, _ := db.GetFailureIDByAnalysis("jira-001")

	err := db.UpsertJiraTicket(&models.JiraTicket{
		FailureID: failureID,
		TicketKey: "PROJ-123",
		TicketURL: "https://jira.example.com/browse/PROJ-123",
		Summary:   "Build failure in auth",
		Status:    "Open",
		Assignee:  "backend",
	})
	if err != nil {
		t.Fatalf("upsert jira: %v", err)
	}

	tickets, err := db.ListPendingJiraTickets()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(tickets))
	}
	if tickets[0].TicketKey != "PROJ-123" {
		t.Errorf("expected PROJ-123, got %q", tickets[0].TicketKey)
	}

	// Mark as resolved — should no longer be pending
	db.UpsertJiraTicket(&models.JiraTicket{
		FailureID: failureID,
		TicketKey: "PROJ-123",
		Status:    "Resolved",
	})

	tickets, _ = db.ListPendingJiraTickets()
	if len(tickets) != 0 {
		t.Errorf("expected 0 pending tickets after resolve, got %d", len(tickets))
	}
}

func TestCountByPlatform(t *testing.T) {
	db := testDB(t)

	db.InsertFailure(&models.Failure{AnalysisID: "c1", Platform: "jenkins", FailedAt: time.Now()})
	db.InsertFailure(&models.Failure{AnalysisID: "c2", Platform: "github", FailedAt: time.Now()})
	db.InsertFailure(&models.Failure{AnalysisID: "c3", Platform: "github", FailedAt: time.Now()})

	jenkins, github, err := db.CountByPlatform()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if jenkins != 1 {
		t.Errorf("expected jenkins=1, got %d", jenkins)
	}
	if github != 2 {
		t.Errorf("expected github=2, got %d", github)
	}

	total, _ := db.TotalFailures()
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
}
