package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Compiler struct {
	tectonicBin string
	typstBin    string
}

func NewCompiler(tectonicBin string, typstBin string) *Compiler {
	if tectonicBin == "" {
		tectonicBin = "tectonic"
	}
	if typstBin == "" {
		typstBin = "typst"
	}
	return &Compiler{
		tectonicBin: tectonicBin,
		typstBin:    typstBin,
	}
}

func (c *Compiler) Compile(workDir string, entryFile string) ([]byte, string, error) {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, "", err
	}

	entryFile = filepath.ToSlash(strings.TrimSpace(entryFile))
	if entryFile == "" {
		return nil, "", fmt.Errorf("entry file is required")
	}

	ext := strings.ToLower(filepath.Ext(entryFile))
	pdfFile := strings.TrimSuffix(entryFile, filepath.Ext(entryFile)) + ".pdf"

	var cmd *exec.Cmd
	switch ext {
	case ".tex":
		cmd = exec.Command(c.tectonicBin, "-X", "compile", entryFile)
	case ".typ":
		cmd = exec.Command(c.typstBin, "compile", entryFile, pdfFile)
	default:
		return nil, "", fmt.Errorf("unsupported entry file: %s", entryFile)
	}

	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		if len(output) == 0 {
			output = []byte(err.Error())
		}
		return nil, string(output), err
	}

	pdfPath := filepath.Join(workDir, filepath.FromSlash(pdfFile))
	pdf, readErr := os.ReadFile(pdfPath)
	if readErr != nil {
		return nil, string(output), readErr
	}

	if filepath.ToSlash(pdfFile) != "main.pdf" {
		_ = os.WriteFile(filepath.Join(workDir, "main.pdf"), pdf, 0644)
	}

	return pdf, string(output), nil
}
