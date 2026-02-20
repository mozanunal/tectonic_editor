.PHONY: help dev build css clean

help:
	@echo "Available targets:"
	@echo "  dev    - Run development server with hot reload"
	@echo "  build  - Build production binary"
	@echo "  css    - Compile Tailwind CSS"
	@echo "  clean  - Remove build artifacts"

dev:
	go run ./cmd/server

build:
	go build -o bin/tectonic-web ./cmd/server

css:
	npx tailwindcss -i ./internal/app/static/input.css -o ./internal/app/static/style.css --watch

css-build:
	npx tailwindcss -i ./internal/app/static/input.css -o ./internal/app/static/style.css --minify

clean:
	rm -rf bin/
	rm -f data/latex.db
