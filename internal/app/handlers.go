package app

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PageData struct {
	User     *User
	Projects []Project
	Project  *Project
	Content  string
	Error    string
}

type Project struct {
	ID      string
	UserID  string
	Name    string
	Created string
	Updated string
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.templates.ExecuteTemplate(w, "login.html", nil)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	s.templates.ExecuteTemplate(w, "register.html", nil)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	var user struct {
		ID           string
		Email        string
		PasswordHash string
		Name         sql.NullString
	}

	err := s.db.QueryRow(
		"SELECT id, email, password_hash, name FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name)

	if err != nil || !checkPassword(password, user.PasswordHash) {
		s.templates.ExecuteTemplate(w, "login.html", PageData{Error: "Invalid credentials"})
		return
	}

	token, err := s.createToken(user.ID, user.Email)
	if err != nil {
		s.templates.ExecuteTemplate(w, "login.html", PageData{Error: "Login failed"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	name := r.FormValue("name")

	hash, err := hashPassword(password)
	if err != nil {
		s.templates.ExecuteTemplate(w, "register.html", PageData{Error: "Registration failed"})
		return
	}

	id := uuid.New().String()
	_, err = s.db.Exec(
		"INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)",
		id, email, hash, name,
	)

	if err != nil {
		s.templates.ExecuteTemplate(w, "register.html", PageData{Error: "Email already exists"})
		return
	}

	token, err := s.createToken(id, email)
	if err != nil {
		s.templates.ExecuteTemplate(w, "register.html", PageData{Error: "Registration failed"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleProjectsPage(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())

	rows, err := s.db.Query(
		"SELECT id, user_id, name, created, updated FROM projects WHERE user_id = ? ORDER BY updated DESC",
		user.ID,
	)
	if err != nil {
		http.Error(w, "Failed to load projects", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Created, &p.Updated)
		projects = append(projects, p)
	}

	s.templates.ExecuteTemplate(w, "projects.html", PageData{
		User:     user,
		Projects: projects,
	})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	name := r.FormValue("name")

	if name == "" {
		name = "Untitled Project"
	}

	id := uuid.New().String()
	_, err := s.db.Exec(
		"INSERT INTO projects (id, user_id, name) VALUES (?, ?, ?)",
		id, user.ID, name,
	)

	if err != nil {
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}

	projectDir := filepath.Join(s.projectsDir, id)
	os.MkdirAll(projectDir, 0755)

	defaultTex := `\documentclass{article}
\begin{document}
Hello, World!
\end{document}
`
	os.WriteFile(filepath.Join(projectDir, "main.tex"), []byte(defaultTex), 0644)

	http.Redirect(w, r, "/editor/"+id, http.StatusSeeOther)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	result, err := s.db.Exec(
		"DELETE FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	)
	if err != nil {
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	os.RemoveAll(projectDir)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleEditorPage(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var project Project
	err := s.db.QueryRow(
		"SELECT id, user_id, name, created, updated FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	).Scan(&project.ID, &project.UserID, &project.Name, &project.Created, &project.Updated)

	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	texPath := filepath.Join(s.projectsDir, projectID, "main.tex")
	content, _ := os.ReadFile(texPath)

	s.templates.ExecuteTemplate(w, "editor.html", PageData{
		User:    user,
		Project: &project,
		Content: string(content),
	})
}

func (s *Server) handleCompile(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var exists bool
	s.db.QueryRow(
		"SELECT 1 FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	).Scan(&exists)

	if !exists {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	content := r.FormValue("content")
	workDir := filepath.Join(s.projectsDir, projectID)

	pdf, output, err := s.compiler.Compile(content, workDir)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(output))
		return
	}

	s.db.Exec("UPDATE projects SET updated = datetime('now') WHERE id = ?", projectID)

	w.Header().Set("Content-Type", "application/pdf")
	w.Write(pdf)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var exists bool
	s.db.QueryRow(
		"SELECT 1 FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	).Scan(&exists)

	if !exists {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	content := r.FormValue("content")
	texPath := filepath.Join(s.projectsDir, projectID, "main.tex")

	if err := os.WriteFile(texPath, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}

	s.db.Exec("UPDATE projects SET updated = datetime('now') WHERE id = ?", projectID)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetPDF(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var exists bool
	s.db.QueryRow(
		"SELECT 1 FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	).Scan(&exists)

	if !exists {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	pdfPath := filepath.Join(s.projectsDir, projectID, "main.pdf")
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		http.Error(w, "PDF not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Write(pdf)
}

type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	IsText  bool   `json:"isText"`
}

func isTextFile(name string) bool {
	textExts := []string{".tex", ".bib", ".sty", ".cls", ".txt", ".md", ".bst", ".cfg", ".def"}
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range textExts {
		if ext == e {
			return true
		}
	}
	return false
}

func (s *Server) verifyProjectAccess(r *http.Request) (string, bool) {
	user := getUserFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var exists bool
	s.db.QueryRow(
		"SELECT 1 FROM projects WHERE id = ? AND user_id = ?",
		projectID, user.ID,
	).Scan(&exists)

	return projectID, exists
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasSuffix(name, ".pdf") || strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".aux") {
			continue
		}
		files = append(files, FileInfo{
			Name:   name,
			Path:   name,
			IsDir:  entry.IsDir(),
			Size:   info.Size(),
			IsText: isTextFile(name),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (s *Server) handleCreateFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	filename = filepath.Clean(filename)
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.projectsDir, projectID, filename)

	if _, err := os.Stat(filePath); err == nil {
		http.Error(w, "File already exists", http.StatusConflict)
		return
	}

	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	r.ParseMultipartForm(32 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := filepath.Clean(header.Filename)
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.projectsDir, projectID, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	filename := chi.URLParam(r, "*")
	filename = filepath.Clean(filename)
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.projectsDir, projectID, filename)

	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if isTextFile(filename) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(content)
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	filename := chi.URLParam(r, "*")
	filename = filepath.Clean(filename)
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.projectsDir, projectID, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	content := r.FormValue("content")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	s.db.Exec("UPDATE projects SET updated = datetime('now') WHERE id = ?", projectID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	filename := chi.URLParam(r, "*")
	filename = filepath.Clean(filename)
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if filename == "main.tex" {
		http.Error(w, "Cannot delete main.tex", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.projectsDir, projectID, filename)

	if err := os.Remove(filePath); err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
