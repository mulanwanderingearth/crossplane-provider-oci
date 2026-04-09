//go:build integration
// +build integration

package workflowtemplategenerator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateApigatewayNamespacedIntegration regenerates a subset of templates
// against the real examples tree to ensure namespaced workflows reference
// namespaced examples for dependent resources.
func TestGenerateApigatewayNamespacedIntegration(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Skipf("repo root not available: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "examples", scopeCluster)); err != nil {
		t.Skipf("cluster examples tree not available under %s: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, "examples", scopeNamespaced)); err != nil {
		t.Skipf("namespaced examples tree not available under %s: %v", root, err)
	}

	cfg := NewConfig(root, "v1alpha1")
	cfg.Services = []string{"apigateway"}
	cfg.OutputDir = filepath.Join(os.TempDir(), "workflowtemplates-integration")

	if err := os.RemoveAll(cfg.OutputDir); err != nil {
		t.Fatalf("RemoveAll() error: %v", err)
	}

	if err := Run(cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	t.Logf("Generated templates in %s", cfg.OutputDir)

	namespacedPath := filepath.Join(cfg.OutputDir, scopeNamespaced, "apigateway-namespaced-v1alpha1.yaml")
	data, err := os.ReadFile(namespacedPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", namespacedPath, err)
	}

	content := string(data)
	if strings.Contains(content, "examples/cluster/identity") {
		t.Fatalf("namespaced template still references cluster identity example:\n%s", namespacedPath)
	}
	if strings.Contains(content, "examples/cluster/networking") {
		t.Fatalf("namespaced template still references cluster networking example:\n%s", namespacedPath)
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) &&
			dirExists(filepath.Join(dir, "examples")) &&
			dirExists(filepath.Join(dir, "argo")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return "", fmt.Errorf("unable to locate repository root from %s", wd)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
