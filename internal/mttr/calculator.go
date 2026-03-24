package mttr

import (
	"database/sql"
	"time"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/models"
)

// Calculator computes live MTTR statistics from the database.
type Calculator struct {
	conn *sql.DB
}

// New creates a new MTTR calculator using the given database connection.
func New(conn *sql.DB) *Calculator {
	return &Calculator{conn: conn}
}

// Calculate returns current MTTR statistics.
func (c *Calculator) Calculate() (*models.MTTRStats, error) {
	stats := &models.MTTRStats{
		ByTeam:     make(map[string]float64),
		ByCategory: make(map[string]float64),
	}

	// Overall average
	row := c.conn.QueryRow(`SELECT COALESCE(AVG(mttr_seconds), 0) FROM failures WHERE resolved_at IS NOT NULL`)
	row.Scan(&stats.OverallAvgSeconds)

	// 7-day rolling average
	cutoff7 := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
	row = c.conn.QueryRow(`SELECT COALESCE(AVG(mttr_seconds), 0) FROM failures WHERE resolved_at IS NOT NULL AND failed_at >= ?`, cutoff7)
	row.Scan(&stats.Avg7DaySeconds)

	// 30-day rolling average
	cutoff30 := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	row = c.conn.QueryRow(`SELECT COALESCE(AVG(mttr_seconds), 0) FROM failures WHERE resolved_at IS NOT NULL AND failed_at >= ?`, cutoff30)
	row.Scan(&stats.Avg30DaySeconds)

	// Total resolved / unresolved
	c.conn.QueryRow(`SELECT COUNT(*) FROM failures WHERE resolved_at IS NOT NULL`).Scan(&stats.TotalResolved)
	c.conn.QueryRow(`SELECT COUNT(*) FROM failures WHERE resolved_at IS NULL`).Scan(&stats.TotalUnresolved)

	// Per-team MTTR
	rows, err := c.conn.Query(`SELECT responsible_team, AVG(mttr_seconds) FROM failures WHERE resolved_at IS NOT NULL AND responsible_team != '' GROUP BY responsible_team`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var team string
			var avg float64
			rows.Scan(&team, &avg)
			stats.ByTeam[team] = avg
		}
	}

	// Per-category MTTR
	rows2, err := c.conn.Query(`SELECT category, AVG(mttr_seconds) FROM failures WHERE resolved_at IS NOT NULL AND category != '' GROUP BY category`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var cat string
			var avg float64
			rows2.Scan(&cat, &avg)
			stats.ByCategory[cat] = avg
		}
	}

	return stats, nil
}
