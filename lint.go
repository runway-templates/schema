package schema

import (
	"fmt"
	"strings"

	"github.com/github/go-spdx/v2/spdxexp"
)

// Severity classifies a lint Issue. The lint pass currently emits only
// warnings — none of the checks are universally required by the schema —
// but the type is exposed so callers can filter or upgrade severities.
type Severity int

const (
	SeverityWarning Severity = iota
	SeverityError
)

func (s Severity) String() string {
	if s == SeverityError {
		return "error"
	}
	return "warning"
}

// Issue is one finding from Lint. Path uses dotted notation with array
// indices, e.g. "services[0].image" or "metadata.license".
type Issue struct {
	Path     string
	Message  string
	Severity Severity
}

func (i Issue) String() string {
	return fmt.Sprintf("%s: %s: %s", i.Severity, i.Path, i.Message)
}

// licenseAllowList accepts identifiers that are not on the SPDX list but
// are common enough in the template ecosystem to recognize. "proprietary"
// is the catch-all for closed-source upstreams. Lookup is case-insensitive.
var licenseAllowList = map[string]struct{}{
	"proprietary":        {},
	"sustainableuse-1.0": {},
}

// Lint runs semantic checks against a decoded Template that the JSON Schema
// cannot express. Currently this is SPDX license validation. It returns all
// issues found; an empty slice means clean. Callers decide whether to treat
// warnings as fatal.
func Lint(t *Template) []Issue {
	if t == nil {
		return nil
	}
	var issues []Issue
	issues = append(issues, lintLicense(t)...)
	issues = append(issues, lintMinPlan(t)...)
	return issues
}

// LintFile decodes the template at path and returns its lint issues. It
// does not run schema validation; pair it with ValidateFile when both
// passes are needed.
func LintFile(path string) ([]Issue, error) {
	t, err := DecodeFile(path)
	if err != nil {
		return nil, err
	}
	return Lint(t), nil
}

func lintMinPlan(t *Template) []Issue {
	var issues []Issue
	for i, s := range t.Services {
		if strings.TrimSpace(s.MinPlan) == "" {
			issues = append(issues, Issue{
				Path:     fmt.Sprintf("services[%d].minPlan", i),
				Message:  "no minPlan declared; the platform will assume the cheapest plan that fits, which may surprise users on free or dev tiers",
				Severity: SeverityWarning,
			})
		}
	}
	return issues
}

func lintLicense(t *Template) []Issue {
	lic := strings.TrimSpace(t.Metadata.License)
	if lic == "" {
		return nil
	}
	if _, ok := licenseAllowList[strings.ToLower(lic)]; ok {
		return nil
	}
	ok, invalid := spdxexp.ValidateLicenses([]string{lic})
	if ok {
		return nil
	}
	return []Issue{{
		Path:     "metadata.license",
		Message:  fmt.Sprintf("not a recognized SPDX identifier or expression: %s", strings.Join(invalid, ", ")),
		Severity: SeverityWarning,
	}}
}
