package schema_test

import (
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func issuePaths(issues []schema.Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Path
	}
	return out
}

// Fixtures with valid SPDX identifiers must produce no metadata.license
// warning. Covers the bare-ID lookup path of go-spdx.
func TestLint_ValidSPDXLicenses(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"mariadb.yaml", "meilisearch.yaml", "mongodb.yaml", "valkey.yaml"} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			assert.NotContains(t, issuePaths(schema.Lint(loadFixture(t, f))), "metadata.license")
		})
	}
}

// n8n.yaml ships with SustainableUse-1.0, which is not on the SPDX list
// but is on the local allowlist — so no warning. Same for "proprietary"
// and a case-insensitive variant.
func TestLint_AllowListedLicenses(t *testing.T) {
	t.Parallel()
	for _, lic := range []string{"SustainableUse-1.0", "proprietary", "Proprietary", "sustainableuse-1.0"} {
		t.Run(lic, func(t *testing.T) {
			t.Parallel()
			tmpl := loadFixture(t, "meilisearch.yaml")
			tmpl.Metadata.License = lic
			assert.NotContains(t, issuePaths(schema.Lint(tmpl)), "metadata.license")
		})
	}
}

// A license that is neither SPDX nor allowlisted still produces a warning.
func TestLint_UnknownLicenseWarns(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "meilisearch.yaml")
	tmpl.Metadata.License = "Made-Up-9.9"
	issues := schema.Lint(tmpl)
	var got *schema.Issue
	for i := range issues {
		if issues[i].Path == "metadata.license" {
			got = &issues[i]
		}
	}
	require.NotNil(t, got, "expected SPDX warning")
	assert.Equal(t, schema.SeverityWarning, got.Severity)
	assert.Contains(t, got.Message, "Made-Up-9.9")
}

// SPDX expressions ("A OR B") are accepted by go-spdx.
func TestLint_LicenseExpression(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "meilisearch.yaml")
	tmpl.Metadata.License = "Apache-2.0 OR MIT"
	assert.NotContains(t, issuePaths(schema.Lint(tmpl)), "metadata.license")
}

// An empty license is permitted by the schema and must not warn.
func TestLint_LicenseEmpty(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "meilisearch.yaml")
	tmpl.Metadata.License = ""
	assert.NotContains(t, issuePaths(schema.Lint(tmpl)), "metadata.license")
}

// LintFile is the file-based wrapper; smoke-test it once. n8n.yaml's
// license is allowlisted, so the file should produce no issues.
func TestLintFile_n8n(t *testing.T) {
	t.Parallel()
	issues, err := schema.LintFile(validFixture("n8n.yaml"))
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// A service without minPlan produces a warning so template authors are
// nudged to declare what tier their service realistically needs.
func TestLint_MissingMinPlanWarns(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].MinPlan = ""
	issues := schema.Lint(tmpl)
	var got *schema.Issue
	for i := range issues {
		if issues[i].Path == "services[0].minPlan" {
			got = &issues[i]
		}
	}
	require.NotNil(t, got, "expected minPlan warning")
	assert.Equal(t, schema.SeverityWarning, got.Severity)
}

// Fixtures that already declare minPlan must not warn.
func TestLint_MinPlanDeclared(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"mariadb.yaml", "meilisearch.yaml", "mongodb.yaml", "valkey.yaml", "n8n.yaml"} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			assert.NotContains(t, issuePaths(schema.Lint(loadFixture(t, f))), "services[0].minPlan")
		})
	}
}
