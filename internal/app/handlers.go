package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	compileStartedAt := time.Now()
	pdf, output, err := s.compiler.Compile(content, workDir)
	compileDurationMs := time.Since(compileStartedAt).Milliseconds()
	w.Header().Set("X-Latex-Compile-Ms", strconv.FormatInt(compileDurationMs, 10))
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
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"isDir"`
	Size        int64  `json:"size"`
	IsText      bool   `json:"isText"`
	ContentType string `json:"contentType"`
}

var compilerArtifacts = map[string]struct{}{
	"main.pdf": {},
	"main.log": {},
	"main.aux": {},
}

func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}

	if detected := mime.TypeByExtension(ext); detected != "" {
		mediaType, _, err := mime.ParseMediaType(detected)
		if err == nil {
			if strings.HasPrefix(mediaType, "text/") {
				return true
			}
			switch mediaType {
			case "application/json", "application/javascript", "application/xml", "application/x-tex":
				return true
			}
		}
	}

	textExts := []string{
		".tex", ".ltx", ".bib", ".sty", ".cls", ".txt", ".md", ".bst", ".cfg", ".def",
		".json", ".yaml", ".yml", ".xml", ".csv", ".tsv", ".html", ".htm", ".css", ".js", ".ts",
		".go", ".py", ".sh", ".toml", ".ini",
	}
	for _, candidate := range textExts {
		if ext == candidate {
			return true
		}
	}

	return false
}

func contentTypeForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		if isTextFile(name) {
			return "text/plain; charset=utf-8"
		}
		return "application/octet-stream"
	}

	if detected := mime.TypeByExtension(ext); detected != "" {
		if strings.HasPrefix(strings.ToLower(detected), "text/") && !strings.Contains(strings.ToLower(detected), "charset=") {
			return detected + "; charset=utf-8"
		}
		return detected
	}

	if isTextFile(name) {
		return "text/plain; charset=utf-8"
	}

	return "application/octet-stream"
}

func detectContentType(name string, content []byte) string {
	byName := contentTypeForName(name)
	if byName != "application/octet-stream" {
		return byName
	}

	if len(content) == 0 {
		return byName
	}

	detected := http.DetectContentType(content)
	if strings.HasPrefix(strings.ToLower(detected), "text/") && !strings.Contains(strings.ToLower(detected), "charset=") {
		return detected + "; charset=utf-8"
	}
	return detected
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

func normalizeProjectPath(input string, allowEmpty bool) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(input, "\\", "/"))
	if value == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errors.New("path required")
	}
	if strings.HasPrefix(value, "/") {
		return "", errors.New("absolute paths are not allowed")
	}

	parts := strings.Split(value, "/")
	normalizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", errors.New("invalid path")
		}
		normalizedParts = append(normalizedParts, part)
	}

	normalized := path.Clean(strings.Join(normalizedParts, "/"))
	if normalized == "." {
		normalized = ""
	}
	if normalized == "" && !allowEmpty {
		return "", errors.New("path required")
	}
	return normalized, nil
}

func isPathWithinProject(projectDir, candidatePath string) bool {
	rel, err := filepath.Rel(projectDir, candidatePath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func resolveProjectPath(projectDir, input string, allowEmpty bool) (string, string, error) {
	rel, err := normalizeProjectPath(input, allowEmpty)
	if err != nil {
		return "", "", err
	}

	abs := projectDir
	if rel != "" {
		abs = filepath.Join(projectDir, filepath.FromSlash(rel))
	}
	if !isPathWithinProject(projectDir, abs) {
		return "", "", errors.New("invalid path")
	}
	return rel, abs, nil
}

func hasHiddenSegment(rel string) bool {
	for _, segment := range strings.Split(rel, "/") {
		if segment != "" && strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func shouldSkipFileListEntry(rel string, isDir bool) bool {
	if hasHiddenSegment(rel) {
		return true
	}
	if isDir {
		return false
	}
	_, isArtifact := compilerArtifacts[rel]
	return isArtifact
}

func (s *Server) touchProject(projectID string) {
	s.db.Exec("UPDATE projects SET updated = datetime('now') WHERE id = ?", projectID)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	var files []FileInfo
	err := filepath.WalkDir(projectDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == projectDir {
			return nil
		}

		relPath, err := filepath.Rel(projectDir, currentPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if shouldSkipFileListEntry(relPath, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		contentType := ""
		isText := false
		if !entry.IsDir() {
			contentType = contentTypeForName(relPath)
			isText = isTextFile(relPath)
		}

		files = append(files, FileInfo{
			Name:        relPath,
			Path:        relPath,
			IsDir:       entry.IsDir(),
			Size:        info.Size(),
			IsText:      isText,
			ContentType: contentType,
		})

		return nil
	})
	if err != nil {
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Path < files[j].Path
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

	projectDir := filepath.Join(s.projectsDir, projectID)
	_, filePath, err := resolveProjectPath(projectDir, r.FormValue("filename"), false)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(filePath); err == nil {
		http.Error(w, "File already exists", http.StatusConflict)
		return
	}

	entryType := strings.ToLower(strings.TrimSpace(r.FormValue("type")))
	if entryType == "dir" {
		if err := os.MkdirAll(filePath, 0755); err != nil {
			http.Error(w, "Failed to create folder", http.StatusInternalServerError)
			return
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}
	}

	s.touchProject(projectID)
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

	projectDir := filepath.Join(s.projectsDir, projectID)
	_, filePath, err := resolveProjectPath(projectDir, header.Filename, false)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		http.Error(w, "Failed to create file path", http.StatusInternalServerError)
		return
	}

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

	s.touchProject(projectID)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	filename, filePath, err := resolveProjectPath(projectDir, chi.URLParam(r, "*"), false)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if fileInfo.IsDir() {
		http.Error(w, "Cannot read directory", http.StatusBadRequest)
		return
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", detectContentType(filename, content))
	w.Write(content)
}

func (s *Server) handleUpdateFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	_, filePath, err := resolveProjectPath(projectDir, chi.URLParam(r, "*"), false)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to access file", http.StatusInternalServerError)
		return
	}
	if fileInfo.IsDir() {
		http.Error(w, "Cannot edit directory", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	s.touchProject(projectID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)
	filename, filePath, err := resolveProjectPath(projectDir, chi.URLParam(r, "*"), false)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if filename == "main.tex" {
		http.Error(w, "Cannot delete main.tex", http.StatusBadRequest)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to access file", http.StatusInternalServerError)
		return
	}

	if fileInfo.IsDir() {
		err = os.RemoveAll(filePath)
	} else {
		err = os.Remove(filePath)
	}
	if err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	s.touchProject(projectID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMoveFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := s.verifyProjectAccess(r)
	if !ok {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	projectDir := filepath.Join(s.projectsDir, projectID)

	sourceRel, sourcePath, err := resolveProjectPath(projectDir, r.FormValue("source"), false)
	if err != nil {
		http.Error(w, "Invalid source path", http.StatusBadRequest)
		return
	}
	if sourceRel == "main.tex" {
		http.Error(w, "Cannot move main.tex", http.StatusBadRequest)
		return
	}

	sourceInfo, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to access source path", http.StatusInternalServerError)
		return
	}

	targetDirRel, targetDirPath, err := resolveProjectPath(projectDir, r.FormValue("targetDir"), true)
	if err != nil {
		http.Error(w, "Invalid target path", http.StatusBadRequest)
		return
	}
	if targetDirRel != "" {
		targetInfo, err := os.Stat(targetDirPath)
		if os.IsNotExist(err) {
			http.Error(w, "Target directory not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "Failed to access target path", http.StatusInternalServerError)
			return
		}
		if !targetInfo.IsDir() {
			http.Error(w, "Target must be a directory", http.StatusBadRequest)
			return
		}
	}

	if sourceInfo.IsDir() && (targetDirRel == sourceRel || strings.HasPrefix(targetDirRel+"/", sourceRel+"/")) {
		http.Error(w, "Cannot move a directory into itself", http.StatusBadRequest)
		return
	}

	targetRel := path.Join(targetDirRel, path.Base(sourceRel))
	if targetRel == sourceRel {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": targetRel})
		return
	}

	targetPath := filepath.Join(projectDir, filepath.FromSlash(targetRel))
	if !isPathWithinProject(projectDir, targetPath) {
		http.Error(w, "Invalid target path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(targetPath); err == nil {
		http.Error(w, "Target already exists", http.StatusConflict)
		return
	} else if !os.IsNotExist(err) {
		http.Error(w, "Failed to access target path", http.StatusInternalServerError)
		return
	}

	if err := os.Rename(sourcePath, targetPath); err != nil {
		http.Error(w, "Failed to move file", http.StatusInternalServerError)
		return
	}

	s.touchProject(projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": targetRel})
}
