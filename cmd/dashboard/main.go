package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PrabhaharanNM/mcp-dashboard/internal/database"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/handlers"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/importer"
	"github.com/PrabhaharanNM/mcp-dashboard/internal/mttr"
	"github.com/PrabhaharanNM/mcp-dashboard/web"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "mcp-dashboard.db", "SQLite database path")
	importDir := flag.String("import-dir", "", "Directory to poll for JSON results (optional)")
	flag.Parse()

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	calc := mttr.New(db.Conn())
	pageHandler := handlers.NewPageHandler(web.TemplateFS)
	apiHandler := handlers.NewAPIHandler(db, calc)
	ingestHandler := handlers.NewIngestHandler(db)

	// File importer (optional)
	if *importDir != "" {
		fi := importer.New(db, *importDir)
		fi.Start()
		defer fi.Stop()
		log.Printf("File importer watching: %s", *importDir)
	}
	// Also check env var
	if envDir := os.Getenv("MCP_IMPORT_DIR"); envDir != "" && *importDir == "" {
		fi := importer.New(db, envDir)
		fi.Start()
		defer fi.Stop()
		log.Printf("File importer watching (env): %s", envDir)
	}

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("/", pageHandler.Dashboard)

	// API
	mux.HandleFunc("/api/dashboard", apiHandler.Dashboard)
	mux.HandleFunc("/api/failures", apiHandler.Failures)
	mux.HandleFunc("/api/mttr", apiHandler.MTTR)
	mux.HandleFunc("/api/pipelines", apiHandler.Pipelines)
	mux.HandleFunc("/api/jira/pending", apiHandler.PendingJira)

	// Ingest
	mux.HandleFunc("/api/ingest/jenkins", ingestHandler.IngestJenkins)
	mux.HandleFunc("/api/ingest/github", ingestHandler.IngestGithub)

	// Resolve uses a path pattern: /api/failures/{id}/resolve
	mux.HandleFunc("/api/failures/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/resolve") {
			ingestHandler.ResolveFailure(w, r)
			return
		}
		http.NotFound(w, r)
	})

	log.Printf("MCP Dashboard starting on %s", *addr)
	log.Printf("Database: %s", *dbPath)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
