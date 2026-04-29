package schema_test

import (
	"path/filepath"
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/require"
)

const (
	validFixturesDir   = "testdata/valid"
	invalidFixturesDir = "testdata/invalid"
)

func validFixture(name string) string   { return filepath.Join(validFixturesDir, name) }
func invalidFixture(name string) string { return filepath.Join(invalidFixturesDir, name) }

// loadFixture decodes a template under testdata/valid into the typed model.
// Use this whenever a test needs a *Template; for tests that need the raw
// path (e.g. ValidateFile, LintFile), call validFixture instead.
func loadFixture(t *testing.T, name string) *schema.Template {
	t.Helper()
	tmpl, err := schema.DecodeFile(validFixture(name))
	require.NoError(t, err)
	return tmpl
}
