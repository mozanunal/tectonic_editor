package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

type Compiler struct {
	binary string
}

func NewCompiler(binary string) *Compiler {
	if binary == "" {
		binary = "tectonic"
	}
	return &Compiler{binary: binary}
}

func (c *Compiler) Compile(latex string, workDir string) ([]byte, string, error) {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, "", err
	}

	texFile := filepath.Join(workDir, "main.tex")
	if err := os.WriteFile(texFile, []byte(latex), 0644); err != nil {
		return nil, "", err
	}

	cmd := exec.Command(c.binary, "-X", "compile", "main.tex")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, string(output), err
	}

	pdfPath := filepath.Join(workDir, "main.pdf")
	pdf, readErr := os.ReadFile(pdfPath)
	if readErr != nil {
		return nil, string(output), readErr
	}

	return pdf, string(output), nil
}
