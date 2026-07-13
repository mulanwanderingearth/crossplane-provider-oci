package noforkpatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type validationRule struct {
	name    string
	paths   []string
	pattern *regexp.Regexp
}

// ValidatePatchedTree performs semantic checks that protect the no-fork runtime
// from upstream provider-global state hazards.
func ValidatePatchedTree(providerDir string) error {
	rules := []validationRule{
		{
			name: "unsafe ConfigureClientVar call site",
			paths: []string{
				"internal/provider",
				"internal/service",
				"internal/client",
			},
			pattern: regexp.MustCompile(`ConfigureClientVar[[:space:]]*\(`),
		},
		{
			name: "unsafe ConfigureClientVar assignment",
			paths: []string{
				"internal/provider",
				"internal/service",
				"internal/client",
			},
			pattern: regexp.MustCompile(`tf_client\.ConfigureClientVar[[:space:]]*=`),
		},
		{
			name: "service-level retry global mutation",
			paths: []string{
				"internal/service",
			},
			pattern: regexp.MustCompile(`tfresource\.(ShortRetryTime|LongRetryTime|ConfiguredRetryDuration)[[:space:]]*=`),
		},
		{
			name: "provider-global tfresource mutation",
			paths: []string{
				"internal/provider",
			},
			pattern: regexp.MustCompile(`^[[:space:]]*tf_resource\.(DefinedTagsToSuppress|RealmSpecificServiceEndpointTemplateEnabled|DualStackEndpointTemplateEnabled|ShortRetryTime|LongRetryTime|ConfiguredRetryDuration)[[:space:]]*=`),
		},
		{
			name: "provider-global retry config mutation",
			paths: []string{
				"internal/provider",
			},
			pattern: regexp.MustCompile(`^[[:space:]]*.*tf_resource\.SetRetriesConfig[[:space:]]*\(`),
		},
		{
			name: "provider-global delete wait mutation",
			paths: []string{
				"internal/provider",
			},
			pattern: regexp.MustCompile(`^[[:space:]]*AvoidWaitingForDeleteTarget[[:space:]]*=`),
		},
		{
			name: "runtime environment mutation",
			paths: []string{
				"internal/provider",
				"internal/service",
				"internal/client",
			},
			pattern: regexp.MustCompile(`^[[:space:]]*.*os\.Setenv[[:space:]]*\(`),
		},
	}

	var errs []error
	for _, rule := range rules {
		findings, err := findMatches(providerDir, rule)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(findings) != 0 {
			errs = append(errs, fmt.Errorf("%s after patch:\n%s", rule.name, strings.Join(findings, "\n")))
		}
	}
	return errors.Join(errs...)
}

func findMatches(providerDir string, rule validationRule) ([]string, error) {
	var findings []string
	for _, rel := range rule.paths {
		root := filepath.Join(providerDir, rel)
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if rule.pattern.MatchString(line) {
					displayPath, err := filepath.Rel(providerDir, path)
					if err != nil {
						displayPath = path
					}
					findings = append(findings, fmt.Sprintf("%s:%d:%s", displayPath, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", root, err)
		}
	}
	return findings, nil
}
