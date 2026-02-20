package app

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) setupRoutes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	staticContent, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Get("/register", s.handleRegisterPage)
	r.Post("/register", s.handleRegister)
	r.Post("/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Get("/", s.handleProjectsPage)
		r.Post("/projects", s.handleCreateProject)
		r.Delete("/projects/{id}", s.handleDeleteProject)

		r.Get("/editor/{id}", s.handleEditorPage)
		r.Post("/compile/{id}", s.handleCompile)
		r.Post("/save/{id}", s.handleSave)
		r.Get("/pdf/{id}", s.handleGetPDF)

		r.Get("/api/projects/{id}/files", s.handleListFiles)
		r.Post("/api/projects/{id}/files", s.handleCreateFile)
		r.Post("/api/projects/{id}/upload", s.handleUploadFile)
		r.Get("/api/projects/{id}/files/*", s.handleGetFile)
		r.Put("/api/projects/{id}/files/*", s.handleUpdateFile)
		r.Delete("/api/projects/{id}/files/*", s.handleDeleteFile)
	})

	return r
}
