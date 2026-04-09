package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	origFlagSet := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	defer func() { flag.CommandLine = origFlagSet }()

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "examples", "cluster", "dns", "v1alpha1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "examples", "cluster", "dns", "v1alpha1", "zone.yaml"), []byte("kind: Zone\nmetadata:\n  labels:\n    testing.upbound.io/example-name: ex-zone\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"cmd", "v1alpha1"}
	if err := run(); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	expectedPath := filepath.Join(root, "argo", "workflowtemplates", "generated-workflowtemplates", "cluster", "dns-cluster-v1alpha1.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("run() expected output file: %v", err)
	}
}
