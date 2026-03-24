package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
)

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// Open opens a SQLite database and runs migrations.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying *sql.DB connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS pipelines (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		platform      TEXT NOT NULL,
		pipeline_name TEXT NOT NULL,
		repository    TEXT NOT NULL DEFAULT '',
		last_status   TEXT NOT NULL DEFAULT 'unknown',
		last_build_at TEXT,
		UNIQUE(platform, pipeline_name)
	);

	CREATE TABLE IF NOT EXISTS failures (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		analysis_id       TEXT NOT NULL UNIQUE,
		platform          TEXT NOT NULL,
		pipeline_id       INTEGER REFERENCES pipelines(id),
		build_identifier  TEXT NOT NULL DEFAULT '',
		build_url         TEXT NOT NULL DEFAULT '',
		job_name          TEXT NOT NULL DEFAULT '',
		build_number      INTEGER NOT NULL DEFAULT 0,
		branch            TEXT NOT NULL DEFAULT '',
		commit_hash       TEXT NOT NULL DEFAULT '',
		failed_stage      TEXT NOT NULL DEFAULT '',
		owner             TEXT NOT NULL DEFAULT '',
		repo              TEXT NOT NULL DEFAULT '',
		workflow          TEXT NOT NULL DEFAULT '',
		run_id            INTEGER NOT NULL DEFAULT 0,
		run_number        INTEGER NOT NULL DEFAULT 0,
		actor             TEXT NOT NULL DEFAULT '',
		sha               TEXT NOT NULL DEFAULT '',
		ref               TEXT NOT NULL DEFAULT '',
		failed_step       TEXT NOT NULL DEFAULT '',
		failed_job        TEXT NOT NULL DEFAULT '',
		status            TEXT NOT NULL DEFAULT 'completed',
		category          TEXT NOT NULL DEFAULT '',
		root_cause_summary TEXT NOT NULL DEFAULT '',
		root_cause_details TEXT NOT NULL DEFAULT '',
		responsible_team  TEXT NOT NULL DEFAULT '',
		team_email        TEXT NOT NULL DEFAULT '',
		confidence        TEXT NOT NULL DEFAULT '',
		evidence          TEXT NOT NULL DEFAULT '[]',
		next_steps        TEXT NOT NULL DEFAULT '[]',
		error_messages    TEXT NOT NULL DEFAULT '[]',
		analysis_time_ms  INTEGER NOT NULL DEFAULT 0,
		jira_ticket_key   TEXT NOT NULL DEFAULT '',
		jira_ticket_url   TEXT NOT NULL DEFAULT '',
		github_issue_url  TEXT NOT NULL DEFAULT '',
		failed_at         TEXT NOT NULL,
		resolved_at       TEXT,
		mttr_seconds      INTEGER,
		developer         TEXT NOT NULL DEFAULT '',
		created_at        TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE INDEX IF NOT EXISTS idx_failures_platform ON failures(platform);
	CREATE INDEX IF NOT EXISTS idx_failures_team ON failures(responsible_team);
	CREATE INDEX IF NOT EXISTS idx_failures_category ON failures(category);
	CREATE INDEX IF NOT EXISTS idx_failures_failed_at ON failures(failed_at);

	CREATE TABLE IF NOT EXISTS jira_tickets (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		failure_id      INTEGER REFERENCES failures(id),
		ticket_key      TEXT NOT NULL UNIQUE,
		ticket_url      TEXT NOT NULL DEFAULT '',
		summary         TEXT NOT NULL DEFAULT '',
		status          TEXT NOT NULL DEFAULT 'Open',
		assignee        TEXT NOT NULL DEFAULT '',
		created_at      TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// UpsertPipeline inserts or updates a pipeline record.
func (db *DB) UpsertPipeline(p *models.Pipeline) (int64, error) {
	var buildAt *string
	if p.LastBuildAt != nil {
		s := p.LastBuildAt.Format(time.RFC3339)
		buildAt = &s
	}
	res, err := db.conn.Exec(`
		INSERT INTO pipelines (platform, pipeline_name, repository, last_status, last_build_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(platform, pipeline_name) DO UPDATE SET
			last_status = excluded.last_status,
			last_build_at = excluded.last_build_at,
			repository = excluded.repository
	`, p.Platform, p.PipelineName, p.Repository, p.LastStatus, buildAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		row := db.conn.QueryRow(`SELECT id FROM pipelines WHERE platform=? AND pipeline_name=?`, p.Platform, p.PipelineName)
		row.Scan(&id)
	}
	return id, nil
}

// ListPipelines returns all pipelines.
func (db *DB) ListPipelines() ([]models.Pipeline, error) {
	rows, err := db.conn.Query(`SELECT id, platform, pipeline_name, repository, last_status, last_build_at FROM pipelines ORDER BY last_build_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pipelines []models.Pipeline
	for rows.Next() {
		var p models.Pipeline
		var buildAt sql.NullString
		if err := rows.Scan(&p.ID, &p.Platform, &p.PipelineName, &p.Repository, &p.LastStatus, &buildAt); err != nil {
			return nil, err
		}
		if buildAt.Valid {
			t, _ := time.Parse(time.RFC3339, buildAt.String)
			p.LastBuildAt = &t
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}

// InsertFailure inserts a failure record. Ignores duplicates by analysis_id.
func (db *DB) InsertFailure(f *models.Failure) error {
	evidence, _ := json.Marshal(f.Evidence)
	nextSteps, _ := json.Marshal(f.NextSteps)
	errorMsgs, _ := json.Marshal(f.ErrorMessages)

	var resolvedAt *string
	if f.ResolvedAt != nil {
		s := f.ResolvedAt.Format(time.RFC3339)
		resolvedAt = &s
	}

	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO failures (
			analysis_id, platform, pipeline_id, build_identifier, build_url,
			job_name, build_number, branch, commit_hash, failed_stage,
			owner, repo, workflow, run_id, run_number, actor, sha, ref, failed_step, failed_job,
			status, category, root_cause_summary, root_cause_details,
			responsible_team, team_email, confidence,
			evidence, next_steps, error_messages, analysis_time_ms,
			jira_ticket_key, jira_ticket_url, github_issue_url,
			failed_at, resolved_at, mttr_seconds, developer
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		f.AnalysisID, f.Platform, f.PipelineID, f.BuildIdentifier, f.BuildURL,
		f.JobName, f.BuildNumber, f.Branch, f.CommitHash, f.FailedStage,
		f.Owner, f.Repo, f.Workflow, f.RunID, f.RunNumber, f.Actor, f.SHA, f.Ref, f.FailedStep, f.FailedJob,
		f.Status, f.Category, f.RootCauseSummary, f.RootCauseDetails,
		f.ResponsibleTeam, f.TeamEmail, f.Confidence,
		string(evidence), string(nextSteps), string(errorMsgs), f.AnalysisTimeMs,
		f.JiraTicketKey, f.JiraTicketURL, f.GithubIssueURL,
		f.FailedAt.Format(time.RFC3339), resolvedAt, f.MTTRSeconds, f.Developer,
	)
	return err
}

// ListFailures returns failures with optional filters.
func (db *DB) ListFailures(limit, offset int, platform, team, category string) ([]models.Failure, error) {
	query := `SELECT id, analysis_id, platform, pipeline_id, build_identifier, build_url,
		job_name, build_number, branch, commit_hash, failed_stage,
		owner, repo, workflow, run_id, run_number, actor, sha, ref, failed_step, failed_job,
		status, category, root_cause_summary, root_cause_details,
		responsible_team, team_email, confidence,
		evidence, next_steps, error_messages, analysis_time_ms,
		jira_ticket_key, jira_ticket_url, github_issue_url,
		failed_at, resolved_at, mttr_seconds, developer, created_at, updated_at
		FROM failures WHERE 1=1`
	var args []interface{}

	if platform != "" {
		query += " AND platform=?"
		args = append(args, platform)
	}
	if team != "" {
		query += " AND responsible_team=?"
		args = append(args, team)
	}
	if category != "" {
		query += " AND category=?"
		args = append(args, category)
	}

	query += " ORDER BY failed_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var failures []models.Failure
	for rows.Next() {
		f, err := scanFailure(rows)
		if err != nil {
			return nil, err
		}
		failures = append(failures, *f)
	}
	return failures, nil
}

func scanFailure(rows *sql.Rows) (*models.Failure, error) {
	var f models.Failure
	var evidenceStr, nextStepsStr, errorMsgsStr string
	var failedAtStr, createdAtStr, updatedAtStr string
	var resolvedAtStr sql.NullString
	var mttrSec sql.NullInt64

	err := rows.Scan(
		&f.ID, &f.AnalysisID, &f.Platform, &f.PipelineID, &f.BuildIdentifier, &f.BuildURL,
		&f.JobName, &f.BuildNumber, &f.Branch, &f.CommitHash, &f.FailedStage,
		&f.Owner, &f.Repo, &f.Workflow, &f.RunID, &f.RunNumber, &f.Actor, &f.SHA, &f.Ref, &f.FailedStep, &f.FailedJob,
		&f.Status, &f.Category, &f.RootCauseSummary, &f.RootCauseDetails,
		&f.ResponsibleTeam, &f.TeamEmail, &f.Confidence,
		&evidenceStr, &nextStepsStr, &errorMsgsStr, &f.AnalysisTimeMs,
		&f.JiraTicketKey, &f.JiraTicketURL, &f.GithubIssueURL,
		&failedAtStr, &resolvedAtStr, &mttrSec, &f.Developer, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(evidenceStr), &f.Evidence)
	json.Unmarshal([]byte(nextStepsStr), &f.NextSteps)
	json.Unmarshal([]byte(errorMsgsStr), &f.ErrorMessages)
	f.FailedAt, _ = time.Parse(time.RFC3339, failedAtStr)
	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	f.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
	if resolvedAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, resolvedAtStr.String)
		f.ResolvedAt = &t
	}
	if mttrSec.Valid {
		f.MTTRSeconds = &mttrSec.Int64
	}

	return &f, nil
}

// ResolveFailure marks a failure as resolved and computes MTTR.
func (db *DB) ResolveFailure(analysisID string, resolvedAt time.Time) error {
	_, err := db.conn.Exec(`
		UPDATE failures SET
			resolved_at = ?,
			mttr_seconds = CAST((julianday(?) - julianday(failed_at)) * 86400 AS INTEGER),
			updated_at = datetime('now')
		WHERE analysis_id = ? AND resolved_at IS NULL
	`, resolvedAt.Format(time.RFC3339), resolvedAt.Format(time.RFC3339), analysisID)
	return err
}

// TeamDistribution returns failure count per team.
func (db *DB) TeamDistribution() (map[string]int, error) {
	rows, err := db.conn.Query(`SELECT responsible_team, COUNT(*) FROM failures WHERE responsible_team != '' GROUP BY responsible_team ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var team string
		var count int
		rows.Scan(&team, &count)
		m[team] = count
	}
	return m, nil
}

// CategoryDistribution returns failure count per category.
func (db *DB) CategoryDistribution() (map[string]int, error) {
	rows, err := db.conn.Query(`SELECT category, COUNT(*) FROM failures WHERE category != '' GROUP BY category ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var cat string
		var count int
		rows.Scan(&cat, &count)
		m[cat] = count
	}
	return m, nil
}

// CountByPlatform returns the count of failures per platform.
func (db *DB) CountByPlatform() (jenkins, github int, err error) {
	row := db.conn.QueryRow(`SELECT COALESCE(SUM(CASE WHEN platform='jenkins' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN platform='github' THEN 1 ELSE 0 END),0) FROM failures`)
	err = row.Scan(&jenkins, &github)
	return
}

// TotalFailures returns the total number of failures.
func (db *DB) TotalFailures() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM failures`).Scan(&count)
	return count, err
}

// UpsertJiraTicket inserts or updates a Jira ticket.
func (db *DB) UpsertJiraTicket(t *models.JiraTicket) error {
	_, err := db.conn.Exec(`
		INSERT INTO jira_tickets (failure_id, ticket_key, ticket_url, summary, status, assignee)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ticket_key) DO UPDATE SET
			status = excluded.status,
			assignee = excluded.assignee,
			updated_at = datetime('now')
	`, t.FailureID, t.TicketKey, t.TicketURL, t.Summary, t.Status, t.Assignee)
	return err
}

// ListPendingJiraTickets returns Jira tickets not yet resolved/closed.
func (db *DB) ListPendingJiraTickets() ([]models.JiraTicket, error) {
	rows, err := db.conn.Query(`
		SELECT id, failure_id, ticket_key, ticket_url, summary, status, assignee, created_at, updated_at
		FROM jira_tickets WHERE status NOT IN ('Resolved', 'Closed', 'Done')
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.JiraTicket
	for rows.Next() {
		var t models.JiraTicket
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.FailureID, &t.TicketKey, &t.TicketURL, &t.Summary, &t.Status, &t.Assignee, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tickets = append(tickets, t)
	}
	return tickets, nil
}

// GetFailureIDByAnalysis returns the failure row ID for a given analysis_id.
func (db *DB) GetFailureIDByAnalysis(analysisID string) (int64, error) {
	var id int64
	err := db.conn.QueryRow(`SELECT id FROM failures WHERE analysis_id=?`, analysisID).Scan(&id)
	return id, err
}

// DistinctTeams returns all unique team names.
func (db *DB) DistinctTeams() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT responsible_team FROM failures WHERE responsible_team != '' ORDER BY responsible_team`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		teams = append(teams, t)
	}
	return teams, nil
}

// DistinctCategories returns all unique category names.
func (db *DB) DistinctCategories() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT category FROM failures WHERE category != '' ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []string
	for rows.Next() {
		var c string
		rows.Scan(&c)
		cats = append(cats, c)
	}
	return cats, nil
}
