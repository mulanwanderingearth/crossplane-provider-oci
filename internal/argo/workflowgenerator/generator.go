package workflowgenerator

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	argoDirName                 = "argo"
	workflowTemplatesDirName    = "workflowtemplates"
	generatedWorkflowTemplates  = "generated-workflowtemplates"
	workflowsDirName            = "workflows"
	generatedWorkflowsDirName   = "generated-workflows"
	defaultParameterMapCapacity = 128
	templateFileName            = "workflow.yaml.tmpl"
	scopeCluster                = "cluster"
	scopeNamespaced             = "namespaced"
	namespaceListParameter      = "namespace_list"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Config contains resolved paths and runtime inputs required by the generator.
type Config struct {
	RootDir      string   // Repository root used to derive default paths.
	Version      string   // Target version suffix used to select workflowtemplates.
	InputDir     string   // Directory containing generated per-service workflowtemplates.
	OutputDir    string   // Directory where consolidated workflows are written.
	ServiceNames []string // Optional service filter applied during workflow generation.
}

// TemplateData is the view model passed to the workflow Go template.
type TemplateData struct {
	Services           []ServiceData // Service DAG tasks included in the final workflow.
	Parameters         []Parameter   // De-duplicated workflow-level parameters.
	HasNamespaced      bool          // Indicates if any namespaced services are present.
	NamespaceListParam string        // Workflow parameter used to supply namespaces.
}

// ServiceData represents one service's workflowtemplate information used by the final DAG.
type ServiceData struct {
	Name               string      // WorkflowTemplate metadata.name.
	TaskName           string      // Generated DAG task name.
	RunParam           string      // Workflow boolean gate for the service task.
	Entrypoint         string      // Referenced template entrypoint.
	Scope              string      // Scope identifier (cluster/namespaced).
	Namespaced         bool        // True if this template must iterate across namespaces.
	NamespaceParameter string      // Parameter name expected by namespaced templates.
	Parameters         []Parameter // WorkflowTemplate argument parameters.
}

// ArgoWorkflowTemplate is a minimal schema used to parse required fields from input YAML files.
type ArgoWorkflowTemplate struct {
	Metadata struct {
		Name string `yaml:"name"` // Referenced WorkflowTemplate name.
	} `yaml:"metadata"`
	Spec struct {
		Entrypoint string `yaml:"entrypoint"` // Template entrypoint invoked by the workflow.
		Arguments  struct {
			Parameters []Parameter `yaml:"parameters"` // WorkflowTemplate argument parameters.
		} `yaml:"arguments"`
	} `yaml:"spec"`
}

// Parameter represents an Argo parameter, optionally with a default value.
type Parameter struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value,omitempty"`
}

// NewConfig builds the default generated-workflow input and output paths under argo/.
func NewConfig(rootDir, version string, serviceNames []string) Config {
	return Config{
		RootDir:      rootDir,
		Version:      version,
		InputDir:     filepath.Join(rootDir, argoDirName, workflowTemplatesDirName, generatedWorkflowTemplates),
		OutputDir:    filepath.Join(rootDir, argoDirName, workflowsDirName, generatedWorkflowsDirName),
		ServiceNames: append([]string(nil), serviceNames...),
	}
}

// withDefaults fills any omitted paths so tests and alternate callers can override selectively.
func (c Config) withDefaults() Config {
	cfg := c
	if cfg.InputDir == "" {
		cfg.InputDir = filepath.Join(cfg.RootDir, argoDirName, workflowTemplatesDirName, generatedWorkflowTemplates)
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(cfg.RootDir, argoDirName, workflowsDirName, generatedWorkflowsDirName)
	}
	return cfg
}

// Run validates configuration, builds template data, and renders the consolidated workflow.
func Run(config Config) error {
	cfg := config.withDefaults()
	if strings.TrimSpace(cfg.Version) == "" {
		return fmt.Errorf("version must not be empty")
	}

	templateDataByScope, err := createTemplateData(cfg)
	if err != nil {
		return err
	}

	return generateWorkflow(cfg, templateDataByScope)
}

// createTemplateData loads workflowtemplate YAML files from each scope and builds sorted, deduplicated template data.
func createTemplateData(config Config) (map[string]TemplateData, error) {
	if err := os.MkdirAll(config.OutputDir, os.ModePerm); err != nil {
		return nil, err
	}

	requestedServices := normalizeRequestedServices(config.ServiceNames)
	templateDataByScope := make(map[string]TemplateData, 2)
	foundAny := false

	for _, scope := range []string{scopeCluster, scopeNamespaced} {
		scopeDir := filepath.Join(config.InputDir, scope)
		scopeEntries, err := os.ReadDir(scopeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		scopeData, err := createTemplateDataForScope(scope, scopeDir, scopeEntries, config.Version, requestedServices)
		if err != nil {
			return nil, err
		}
		if scopeData == nil {
			continue
		}

		templateDataByScope[scope] = *scopeData
		foundAny = true
	}

	if !foundAny {
		return nil, fmt.Errorf("no workflowtemplates found in %s for version %s", config.InputDir, config.Version)
	}

	return templateDataByScope, nil
}

func createTemplateDataForScope(scope, scopeDir string, entries []os.DirEntry, version string, requestedServices []string) (*TemplateData, error) {
	parameterByName := make(map[string]Parameter, defaultParameterMapCapacity)
	services := make([]ServiceData, 0)
	hasNamespaced := scope == scopeNamespaced

	if len(requestedServices) == 0 {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}

			serviceName, entryScope, ok := parseTemplateFileName(entry.Name(), version)
			if !ok || entryScope != scope {
				continue
			}

			templatePath := filepath.Join(scopeDir, entry.Name())
			serviceData, err := processArgoWorkflowTemplate(templatePath, serviceName, scope)
			if err != nil {
				log.Printf("Skipping %q due to error: %v", templatePath, err)
				continue
			}
			services = append(services, serviceData)
			collateParameters(parameterByName, serviceData.Parameters)
			if serviceData.Namespaced {
				hasNamespaced = true
			}
		}
	} else {
		for _, serviceName := range requestedServices {
			fileName := fmt.Sprintf("%s-%s-%s.yaml", serviceName, scope, version)
			templatePath := filepath.Join(scopeDir, fileName)
			if _, err := os.Stat(templatePath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}

			serviceData, err := processArgoWorkflowTemplate(templatePath, serviceName, scope)
			if err != nil {
				log.Printf("Skipping %q due to error: %v", templatePath, err)
				continue
			}
			services = append(services, serviceData)
			collateParameters(parameterByName, serviceData.Parameters)
			if serviceData.Namespaced {
				hasNamespaced = true
			}
		}
	}

	if len(services) == 0 {
		return nil, nil
	}

	if hasNamespaced {
		if _, exists := parameterByName[namespaceListParameter]; !exists {
			parameterByName[namespaceListParameter] = Parameter{
				Name:  namespaceListParameter,
				Value: "[]",
			}
		}
	}

	parameters := make([]Parameter, 0, len(parameterByName))
	for _, parameter := range parameterByName {
		parameters = append(parameters, parameter)
	}
	sort.Slice(parameters, func(i, j int) bool {
		return parameters[i].Name < parameters[j].Name
	})

	sort.Slice(services, func(i, j int) bool {
		return services[i].TaskName < services[j].TaskName
	})

	data := TemplateData{
		Services:           services,
		Parameters:         parameters,
		HasNamespaced:      hasNamespaced,
		NamespaceListParam: namespaceListParameter,
	}
	if !hasNamespaced {
		data.NamespaceListParam = ""
	}
	return &data, nil
}

// normalizeRequestedServices trims and de-duplicates requested service names while preserving order.
func normalizeRequestedServices(serviceNames []string) []string {
	normalized := make([]string, 0, len(serviceNames))
	seen := make(map[string]struct{}, len(serviceNames))
	for _, serviceName := range serviceNames {
		trimmed := strings.TrimSpace(serviceName)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func parseTemplateFileName(fileName, version string) (string, string, bool) {
	if filepath.Ext(fileName) != ".yaml" {
		return "", "", false
	}
	base := strings.TrimSuffix(fileName, ".yaml")
	versionSuffix := "-" + version
	if !strings.HasSuffix(base, versionSuffix) {
		return "", "", false
	}
	base = strings.TrimSuffix(base, versionSuffix)
	lastDash := strings.LastIndex(base, "-")
	if lastDash == -1 {
		return "", "", false
	}

	serviceName := base[:lastDash]
	scope := base[lastDash+1:]
	if serviceName == "" || scope == "" {
		return "", "", false
	}

	switch scope {
	case scopeCluster, scopeNamespaced:
		return serviceName, scope, true
	default:
		return "", "", false
	}
}

// processArgoWorkflowTemplate parses one workflowtemplate and converts it to ServiceData.
func processArgoWorkflowTemplate(workflowTemplatePath, serviceName, scope string) (ServiceData, error) {
	data, err := os.ReadFile(workflowTemplatePath)
	if err != nil {
		return ServiceData{}, err
	}
	var awt ArgoWorkflowTemplate
	if err := yaml.Unmarshal(data, &awt); err != nil {
		return ServiceData{}, fmt.Errorf("failed to parse workflowtemplate %s: %w", workflowTemplatePath, err)
	}

	if strings.TrimSpace(awt.Metadata.Name) == "" {
		return ServiceData{}, fmt.Errorf("workflowtemplate %s has empty metadata.name", workflowTemplatePath)
	}
	if strings.TrimSpace(awt.Spec.Entrypoint) == "" {
		return ServiceData{}, fmt.Errorf("workflowtemplate %s has empty spec.entrypoint", workflowTemplatePath)
	}

	serviceParameters := awt.Spec.Arguments.Parameters
	sort.Slice(serviceParameters, func(i, j int) bool {
		return serviceParameters[i].Name < serviceParameters[j].Name
	})

	// Prefer the requested service name for task and parameter normalization.
	normalizedServiceName := normalizeName(serviceName)
	if normalizedServiceName == "" {
		// Fall back to metadata.name if the filename-derived service name is unusable.
		normalizedServiceName = normalizeName(awt.Metadata.Name)
	}
	if normalizedServiceName == "" {
		return ServiceData{}, fmt.Errorf("unable to derive normalized service name for %s", workflowTemplatePath)
	}

	taskName := strings.ReplaceAll(normalizedServiceName, "_", "-") + "-" + strings.ReplaceAll(scope, "_", "-")
	serviceData := ServiceData{
		Name:               awt.Metadata.Name,
		TaskName:           fmt.Sprintf("%s-tests", taskName),
		RunParam:           fmt.Sprintf("run_%s_%s_tests", normalizedServiceName, scope),
		Entrypoint:         awt.Spec.Entrypoint,
		Scope:              scope,
		Namespaced:         scope == scopeNamespaced,
		NamespaceParameter: "target_namespace",
		Parameters:         serviceParameters,
	}
	if !serviceData.Namespaced {
		serviceData.NamespaceParameter = ""
	}
	return serviceData, nil
}

// collateParameters merges service parameters by name and preserves the first non-empty default value.
func collateParameters(parameterByName map[string]Parameter, parameters []Parameter) {
	for _, parameter := range parameters {
		if parameter.Name == "" {
			continue
		}
		if parameter.Name == "target_namespace" {
			// target_namespace is supplied dynamically for namespaced workflows.
			continue
		}
		existing, ok := parameterByName[parameter.Name]
		if !ok {
			parameterByName[parameter.Name] = parameter
			continue
		}
		if existing.Value == "" && parameter.Value != "" {
			existing.Value = parameter.Value
			parameterByName[parameter.Name] = existing
		}
	}
}

// normalizeName converts a string into a stable snake_case-like identifier for task/parameter naming.
func normalizeName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	lastWasUnderscore := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastWasUnderscore = false
		case !lastWasUnderscore:
			builder.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	return strings.Trim(builder.String(), "_")
}

// resolveWhen renders an Argo "when" expression for the given workflow parameter.
func resolveWhen(parameterName string) string {
	return fmt.Sprintf("{{workflow.parameters.%s}} == true", parameterName)
}

// resolveWorkflowParameter renders an Argo workflow parameter reference expression.
func resolveWorkflowParameter(parameterName string) string {
	return fmt.Sprintf("{{workflow.parameters.%s}}", parameterName)
}

// generateWorkflow renders the consolidated workflow YAML file from template data.
func generateWorkflow(config Config, templateDataByScope map[string]TemplateData) error {
	workflowTemplate, err := template.New(templateFileName).Funcs(template.FuncMap{
		"resolveWhen":      resolveWhen,
		"resolveParameter": resolveWorkflowParameter,
	}).ParseFS(templateFS, "templates/"+templateFileName)
	if err != nil {
		return err
	}

	for _, scope := range []string{scopeCluster, scopeNamespaced} {
		templateData, ok := templateDataByScope[scope]
		if !ok {
			continue
		}

		outputFilePath := filepath.Join(config.OutputDir, fmt.Sprintf("crossplane-provider-oci-%s-%s.yaml", scope, config.Version))
		file, err := os.Create(outputFilePath)
		if err != nil {
			return err
		}

		if err := workflowTemplate.Execute(file, templateData); err != nil {
			file.Close()
			return err
		}

		if err := file.Close(); err != nil {
			return err
		}

		log.Printf("Generated workflow file: %s", outputFilePath)
	}

	return nil
}
