package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/config"
)

// TestCIImageMatrixMatchesContract asserts that .github/images.json — the single
// source for the CI build/publish matrix — lists exactly the images declared in
// the deployment contract. This is the guard that would have caught epp-server,
// whois, and mcp-server being absent from the publish pipeline while present in
// the contract. It fails in either direction: a contract image missing from CI
// (would never be published) or a CI image with no matching service.
func TestCIImageMatrixMatchesContract(t *testing.T) {
	root := findProjectRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, ".github", "images.json"))
	if err != nil {
		t.Fatalf("read .github/images.json: %v", err)
	}
	var entries []struct {
		Name    string `json:"name"`
		Context string `json:"context"`
		File    string `json:"file"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("parse .github/images.json: %v", err)
	}

	ciNames := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Name == "" || e.Context == "" || e.File == "" {
			t.Errorf(".github/images.json entry is missing name/context/file: %+v", e)
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(e.File))); err != nil {
			t.Errorf(".github/images.json: Dockerfile %q for image %q does not exist", e.File, e.Name)
		}
		ciNames[e.Name] = true
	}

	c, err := config.GenerateContract(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("GenerateContract: %v", err)
	}
	contractNames := make(map[string]bool, len(c.Services))
	for _, svc := range c.Services {
		base := svc.Image[strings.LastIndex(svc.Image, "/")+1:]
		contractNames[base] = true
	}

	for name := range contractNames {
		if !ciNames[name] {
			t.Errorf("contract image %q is absent from .github/images.json — CI will not build or publish it. Add it to the matrix.", name)
		}
	}
	for name := range ciNames {
		if !contractNames[name] {
			t.Errorf(".github/images.json lists %q, which is not a contract service — remove it or add the service to serviceMetaRegistry.", name)
		}
	}
}
