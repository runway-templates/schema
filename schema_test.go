package schema_test

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(raw), &v))
	return v
}

func yamlFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".yaml" {
			continue
		}
		out = append(out, e.Name())
	}
	require.NotEmpty(t, out, "no .yaml files found in %s", dir)
	return out
}

// TestValidFixtures asserts every checked-in template under testdata/valid/
// satisfies the schema. New templates added there must validate.
func TestValidFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range yamlFiles(t, validFixturesDir) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, schema.ValidateFile(validFixture(name)))
		})
	}
}

// TestInvalidFixtures asserts every file under testdata/invalid/ fails
// validation. Each file's name describes the violation it demonstrates.
func TestInvalidFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range yamlFiles(t, invalidFixturesDir) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, schema.ValidateFile(invalidFixture(name)),
				"expected validation error")
		})
	}
}

func TestValidateFileMissing(t *testing.T) {
	t.Parallel()
	err := schema.ValidateFile(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// minimalValid is the smallest document that satisfies the schema.
// Negative table tests below mutate one rule at a time so each failure
// isolates a single constraint.
const minimalValid = `{
  "apiVersion": "schema.runway.horse/templates/v1alpha1",
  "kind": "Template",
  "metadata": {
    "name": "demo",
    "version": "1.0.0",
    "displayName": "Demo",
    "description": "A demo template.",
    "category": "other"
  },
  "services": [
    {"name": "web", "image": "nginx:1.27"}
  ]
}`

func TestMinimalValid(t *testing.T) {
	t.Parallel()
	sch, err := schema.Schema()
	require.NoError(t, err)
	require.NoError(t, sch.Validate(decodeJSON(t, minimalValid)))
}

// baseDoc returns the minimalValid document with one mutation applied.
// Each negative test mutates a single field so the resulting validation
// error isolates the rule under test.
func baseDoc(t *testing.T, mutate func(map[string]any)) any {
	t.Helper()
	doc := decodeJSON(t, minimalValid).(map[string]any)
	mutate(doc)
	return doc
}

func TestSchemaRules(t *testing.T) {
	t.Parallel()
	sch, err := schema.Schema()
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name:   "apiVersion must match const",
			mutate: func(d map[string]any) { d["apiVersion"] = "runway.horse/v1beta1" },
		},
		{
			name:   "kind must be Template",
			mutate: func(d map[string]any) { d["kind"] = "Stack" },
		},
		{
			name: "metadata.name rejects uppercase",
			mutate: func(d map[string]any) {
				d["metadata"].(map[string]any)["name"] = "BadName"
			},
		},
		{
			name: "metadata.category enum",
			mutate: func(d map[string]any) {
				d["metadata"].(map[string]any)["category"] = "frobnicator"
			},
		},
		{
			name:   "services minItems is 1",
			mutate: func(d map[string]any) { d["services"] = []any{} },
		},
		{
			name: "service.name must be DNS-1123",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["name"] = "Web"
			},
		},
		{
			name: "service.image cannot be empty",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["image"] = ""
			},
		},
		{
			name: "env keys must be uppercase identifiers",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["env"] = map[string]any{"port": "8080"}
			},
		},
		{
			name: "volume mountPath must be absolute",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["volume"] = map[string]any{"mountPath": "data"}
			},
		},
		{
			name: "boolean input cannot have pattern",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"flag": map[string]any{"type": "boolean", "pattern": "^.+$"},
				}
			},
		},
		{
			name: "string input cannot have min/max",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"name": map[string]any{"type": "string", "min": 1},
				}
			},
		},
		{
			name: "integer input cannot have pattern",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"port": map[string]any{"type": "integer", "pattern": "^[0-9]+$"},
				}
			},
		},
		{
			name: "string input rejects integer default",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"tag": map[string]any{"type": "string", "default": 8080},
				}
			},
		},
		{
			name: "integer input rejects string default",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"port": map[string]any{"type": "integer", "default": "8080"},
				}
			},
		},
		{
			name: "boolean input rejects string default",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"flag": map[string]any{"type": "boolean", "default": "true"},
				}
			},
		},
		{
			name: "input.enum requires at least 2 entries",
			mutate: func(d map[string]any) {
				d["inputs"] = map[string]any{
					"mode": map[string]any{"type": "string", "enum": []any{"only-one"}},
				}
			},
		},
		{
			name: "healthcheck.path must start with /",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["healthcheck"] = map[string]any{
					"type": "http",
					"path": "healthz",
				}
			},
		},
		{
			name:   "additionalProperties at top level rejected",
			mutate: func(d map[string]any) { d["extra"] = true },
		},
		{
			name: "envValue object requires value",
			mutate: func(d map[string]any) {
				d["services"].([]any)[0].(map[string]any)["env"] = map[string]any{
					"PORT": map[string]any{"description": "no value here"},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := sch.Validate(baseDoc(t, tc.mutate))
			assert.Error(t, err, "expected validation error")
		})
	}
}

// TestInputDefaultMatchesType asserts a default of the declared type is
// accepted for each input type.
func TestInputDefaultMatchesType(t *testing.T) {
	t.Parallel()
	sch, err := schema.Schema()
	require.NoError(t, err)

	doc := baseDoc(t, func(d map[string]any) {
		d["inputs"] = map[string]any{
			"tag":  map[string]any{"type": "string", "default": "lts"},
			"port": map[string]any{"type": "integer", "default": 8080},
			"flag": map[string]any{"type": "boolean", "default": true},
		}
	})
	assert.NoError(t, sch.Validate(doc))
}

func TestEnvValueAcceptsBothShapes(t *testing.T) {
	t.Parallel()
	sch, err := schema.Schema()
	require.NoError(t, err)

	shorthand := baseDoc(t, func(d map[string]any) {
		d["services"].([]any)[0].(map[string]any)["env"] = map[string]any{"PORT": "8080"}
	})
	object := baseDoc(t, func(d map[string]any) {
		d["services"].([]any)[0].(map[string]any)["env"] = map[string]any{
			"PORT": map[string]any{"value": "8080", "description": "http port"},
		}
	})

	assert.NoError(t, sch.Validate(shorthand))
	assert.NoError(t, sch.Validate(object))
}
