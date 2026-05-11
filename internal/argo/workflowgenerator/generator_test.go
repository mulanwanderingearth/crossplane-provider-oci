package workflowgenerator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeneratesWorkflow(t *testing.T) {
	root := t.TempDir()
	cfg := NewConfig(root, "v1alpha1", nil)

	writeTestFile(t, filepath.Join(cfg.InputDir, "cluster", "network-cluster-v1alpha1.yaml"), `
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: network-template
spec:
  entrypoint: network-tests
  arguments:
    parameters:
    - name: compartment_ocid
      value: foo
`)
	writeTestFile(t, filepath.Join(cfg.InputDir, "namespaced", "identity-namespaced-v1alpha1.yaml"), `
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: identity-template
spec:
  entrypoint: identity-tests
  arguments:
    parameters:
    - name: tenancy_ocid
`)

	if err := Run(cfg); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	clusterOutputPath := filepath.Join(cfg.OutputDir, "crossplane-provider-oci-cluster-v1alpha1.yaml")
	clusterData, err := os.ReadFile(clusterOutputPath)
	if err != nil {
		t.Fatalf("expected cluster output file: %v", err)
	}

	clusterOutput := string(clusterData)
	if strings.Contains(clusterOutput, "identity-template") {
		t.Fatalf("cluster workflow unexpectedly referenced namespaced service: %s", clusterOutput)
	}
	if !strings.Contains(clusterOutput, "name: run_network_cluster_tests") {
		t.Fatalf("cluster workflow missing network run param: %s", clusterOutput)
	}
	if strings.Contains(clusterOutput, "namespace_list") {
		t.Fatalf("cluster workflow should not declare namespace_list parameter: %s", clusterOutput)
	}

	namespacedOutputPath := filepath.Join(cfg.OutputDir, "crossplane-provider-oci-namespaced-v1alpha1.yaml")
	namespacedData, err := os.ReadFile(namespacedOutputPath)
	if err != nil {
		t.Fatalf("expected namespaced output file: %v", err)
	}

	namespacedOutput := string(namespacedData)
	if !strings.Contains(namespacedOutput, "name: run_identity_namespaced_tests") {
		t.Fatalf("namespaced workflow missing identity run param: %s", namespacedOutput)
	}
	if !strings.Contains(namespacedOutput, "name: namespace_list") {
		t.Fatalf("namespaced workflow missing namespace_list parameter: %s", namespacedOutput)
	}
	if !strings.Contains(namespacedOutput, "{{item}}") {
		t.Fatalf("namespaced workflow missing namespace iteration: %s", namespacedOutput)
	}
}

func TestRunFiltersRequestedServices(t *testing.T) {
	root := t.TempDir()
	cfg := NewConfig(root, "v1alpha1", []string{"network"})

	writeTestFile(t, filepath.Join(cfg.InputDir, "cluster", "network-cluster-v1alpha1.yaml"), `
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: network-template
spec:
  entrypoint: network-tests
`)
	writeTestFile(t, filepath.Join(cfg.InputDir, "namespaced", "network-namespaced-v1alpha1.yaml"), `
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: network-template-namespaced
spec:
  entrypoint: network-tests-namespaced
`)

	if err := Run(cfg); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	clusterOutputPath := filepath.Join(cfg.OutputDir, "crossplane-provider-oci-cluster-v1alpha1.yaml")
	clusterData, err := os.ReadFile(clusterOutputPath)
	if err != nil {
		t.Fatalf("expected cluster output file: %v", err)
	}
	clusterOutput := string(clusterData)
	if strings.Contains(clusterOutput, "identity-template") {
		t.Fatalf("cluster workflow unexpectedly included filtered service: %s", clusterOutput)
	}
	if !strings.Contains(clusterOutput, "network-template") {
		t.Fatalf("cluster workflow missing requested service: %s", clusterOutput)
	}

	namespacedOutputPath := filepath.Join(cfg.OutputDir, "crossplane-provider-oci-namespaced-v1alpha1.yaml")
	if _, err := os.Stat(namespacedOutputPath); err != nil {
		t.Fatalf("expected namespaced output file: %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
