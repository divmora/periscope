package cmd

import (
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/divmora/periscope/internal/api/handlers"
	"github.com/divmora/periscope/internal/auth"
	"github.com/divmora/periscope/internal/store"
	"github.com/divmora/periscope/ui"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Periscope central web server and API",
	Run: func(cmd *cobra.Command, args []string) {
		if err := store.Connect(cmd.Context()); err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		r := chi.NewRouter()

		// A good base middleware stack
		r.Use(middleware.RequestID)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		// Basic CORS setup for local UI development (Vite is on 5173)
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"https://*", "http://*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))

		// Orchestration Probes & Metrics
		r.Get("/healthz", handlers.Healthz)
		r.Get("/readyz", handlers.Readyz)
		r.Handle("/metrics", promhttp.Handler())

		// API Routes
		r.Route("/api", func(r chi.Router) {

			// Public & Session Auth Routes
			r.Route("/auth", func(r chi.Router) {
				r.Post("/login", handlers.Login)
				r.Post("/logout", handlers.Logout)

				// Protected Auth Routes
				r.With(auth.RequireAuth).Get("/me", handlers.Me)
			})

			// Protected Project Routes
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuth)

				// Global System Settings
				r.Get("/system/storage", handlers.GetSystemStorage)

				r.Route("/projects", func(r chi.Router) {
					r.Get("/", handlers.GetProjects)
					r.Post("/", handlers.CreateProject)

					r.Route("/{projectId}", func(r chi.Router) {
						// Ensure user has access to this project (or is global admin)
						r.Use(auth.RequireProjectAccess)

						// Project Configuration (Admin only)
						r.With(auth.RequireProjectAdmin).Get("/config", handlers.GetProjectConfig)
						r.With(auth.RequireProjectAdmin).Put("/config", handlers.UpdateProjectConfig)
						r.With(auth.RequireProjectAdmin).Delete("/", handlers.DeleteProject)

						// Project Memberships (RBAC management)
						r.Route("/members", func(r chi.Router) {
							r.Get("/", handlers.ListProjectMembers)
							r.With(auth.RequireProjectAdmin).Post("/", handlers.AddProjectMember)
							r.With(auth.RequireProjectAdmin).Put("/{userIid}", handlers.UpdateProjectMember)
							r.With(auth.RequireProjectAdmin).Delete("/{userIid}", handlers.RemoveProjectMember)
						})

						// Project Teams & Source-level RBAC (Grafana/Datadog model)
						r.Route("/teams", func(r chi.Router) {
							r.Get("/", handlers.ListProjectTeams)
							r.With(auth.RequireProjectAdmin).Post("/", handlers.CreateProjectTeam)
							r.Route("/{teamId}", func(r chi.Router) {
								r.With(auth.RequireProjectAdmin).Put("/", handlers.UpdateProjectTeam)
								r.With(auth.RequireProjectAdmin).Delete("/", handlers.DeleteProjectTeam)
								r.Get("/members", handlers.ListTeamMembers)
								r.With(auth.RequireProjectAdmin).Post("/members", handlers.AddTeamMember)
								r.With(auth.RequireProjectAdmin).Delete("/members/{userIid}", handlers.RemoveTeamMember)
							})
						})

						// Multi-Source Data Ingestion Sources
						r.Route("/sources", func(r chi.Router) {
							r.Get("/", handlers.ListSources)
							r.With(auth.RequireProjectAdmin).Post("/", handlers.CreateSource)
							r.With(auth.RequireProjectAdmin).Delete("/{sourceIid}", handlers.DeleteSource)
						})

						// Embedded DuckDB SQL Studio
						r.Route("/query", func(r chi.Router) {
							r.Post("/execute", handlers.ExecuteSQLQuery)
						})

						// AWS Aurora Database Activity Streams (Direct S3 / Athena)
						r.Route("/das", func(r chi.Router) {
							r.Get("/activity", handlers.GetDASActivity)
							r.Get("/stats", handlers.GetDASStats)
						})

						// Telescope Telemetry Data
						r.Route("/telescope", func(r chi.Router) {
							r.Get("/requests", handlers.GetRequests)
							r.Get("/exceptions", handlers.GetExceptions)
							r.Get("/queries", handlers.GetQueries)
							r.Get("/logs", handlers.GetLogs)
						})
					})
				})
			})

		})

		// ---------------------------------------------------------
		// UI: Serve Embedded React Frontend
		// ---------------------------------------------------------
		distSubFS, err := fs.Sub(ui.DistFS, "dist")
		if err != nil {
			log.Printf("Warning: Failed to load embedded UI (did you run pnpm build?): %v", err)
		} else {
			fileServer := http.FileServer(http.FS(distSubFS))

			r.Handle("/assets/*", fileServer)
			r.Handle("/favicon.ico", fileServer)

			r.NotFound(func(w http.ResponseWriter, r *http.Request) {
				indexFile, err := distSubFS.Open("index.html")
				if err != nil {
					http.Error(w, "UI not built", http.StatusNotFound)
					return
				}
				defer indexFile.Close()

				w.Header().Set("Content-Type", "text/html")
				stat, _ := indexFile.Stat()
				http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
			})
		}

		log.Println("Starting Periscope server on port 8006...")
		if err := http.ListenAndServe(":8006", r); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
