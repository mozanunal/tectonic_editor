package app

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*.html")
}
