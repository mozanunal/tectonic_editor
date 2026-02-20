package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCompilableSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "tex", filename: "main.tex", want: true},
		{name: "typ", filename: "paper.typ", want: true},
		{name: "uppercase typ", filename: "PAPER.TYP", want: true},
		{name: "bib", filename: "refs.bib", want: false},
		{name: "no ext", filename: "main", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isCompilableSource(tc.filename)
			if got != tc.want {
				t.Fatalf("isCompilableSource(%q)=%v want %v", tc.filename, got, tc.want)
			}
		})
	}
}

func TestDefaultCompileEntry(t *testing.T) {
	t.Parallel()

	t.Run("prefers main tex", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "main.typ"), "#set page(width: 120mm)")
		writeTestFile(t, filepath.Join(dir, "main.tex"), "\\documentclass{article}")

		got, err := defaultCompileEntry(dir)
		if err != nil {
			t.Fatalf("defaultCompileEntry returned error: %v", err)
		}
		if got != "main.tex" {
			t.Fatalf("defaultCompileEntry=%q want %q", got, "main.tex")
		}
	})

	t.Run("uses main typ when tex missing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "main.typ"), "Hello Typst")

		got, err := defaultCompileEntry(dir)
		if err != nil {
			t.Fatalf("defaultCompileEntry returned error: %v", err)
		}
		if got != "main.typ" {
			t.Fatalf("defaultCompileEntry=%q want %q", got, "main.typ")
		}
	})

	t.Run("falls back to lexicographic compilable file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "chapters", "c2.typ"), "B")
		writeTestFile(t, filepath.Join(dir, "chapters", "c1.typ"), "A")

		got, err := defaultCompileEntry(dir)
		if err != nil {
			t.Fatalf("defaultCompileEntry returned error: %v", err)
		}
		if got != "chapters/c1.typ" {
			t.Fatalf("defaultCompileEntry=%q want %q", got, "chapters/c1.typ")
		}
	})

	t.Run("skips hidden directories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, ".hidden", "main.typ"), "hidden")
		writeTestFile(t, filepath.Join(dir, "visible.typ"), "visible")

		got, err := defaultCompileEntry(dir)
		if err != nil {
			t.Fatalf("defaultCompileEntry returned error: %v", err)
		}
		if got != "visible.typ" {
			t.Fatalf("defaultCompileEntry=%q want %q", got, "visible.typ")
		}
	})

	t.Run("fails without compilable files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "notes.txt"), "hello")

		_, err := defaultCompileEntry(dir)
		if err == nil {
			t.Fatalf("expected error when no compile entry exists")
		}
	})
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
