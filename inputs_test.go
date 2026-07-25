package schema_test

import (
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inputIssuePaths(issues []schema.Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Path
	}
	return out
}

// valkey declares no required inputs, so an empty value set is valid and
// the defaults come back normalized.
func TestValidateInputs_DefaultsFilled(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	values, issues := schema.ValidateInputs(tmpl, nil)
	assert.Empty(t, issues)
	assert.Equal(t, "9.0.3-alpine", values["image_tag"])
	assert.Equal(t, "noeviction", values["maxmemory_policy"])
}

func TestValidateInputs_RequiredMissing(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	_, issues := schema.ValidateInputs(tmpl, map[string]any{})
	paths := inputIssuePaths(issues)
	assert.Contains(t, paths, "inputs.public_url")
	assert.Contains(t, paths, "inputs.postgres_host")
	assert.Contains(t, paths, "inputs.redis_password")
	// Optional inputs with defaults must not be reported.
	assert.NotContains(t, paths, "inputs.image_tag")
	assert.NotContains(t, paths, "inputs.postgres_port")
}

func TestValidateInputs_ValidSet(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	values, issues := schema.ValidateInputs(tmpl, map[string]any{
		"public_url":        "https://n8n.example.com",
		"postgres_host":     "db.internal",
		"postgres_database": "n8n",
		"postgres_user":     "n8n",
		"postgres_password": "hunter2hunter2",
		"redis_host":        "valkey.internal",
		"redis_password":    "hunter2hunter2",
		// JSON decoding hands integers over as float64.
		"postgres_port": float64(5433),
	})
	require.Empty(t, issues)
	assert.Equal(t, int64(5433), values["postgres_port"])
	assert.Equal(t, int64(6379), values["redis_port"], "default filled and normalized")
	assert.Equal(t, "https://n8n.example.com", values["public_url"])
}

func TestValidateInputs_UnknownKey(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	_, issues := schema.ValidateInputs(tmpl, map[string]any{"nope": "x"})
	require.Len(t, issues, 1)
	assert.Equal(t, "inputs.nope", issues[0].Path)
	assert.Contains(t, issues[0].Message, "not a declared input")
}

func TestValidateInputs_WrongTypes(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	for name, tc := range map[string]struct {
		key   string
		value any
	}{
		"bool for string":    {"postgres_host", true},
		"string for integer": {"postgres_port", "5432"},
		"fractional for int": {"postgres_port", 54.32},
		"integer for string": {"postgres_user", 42},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, issues := schema.ValidateInputs(tmpl, map[string]any{tc.key: tc.value})
			assert.Contains(t, inputIssuePaths(issues), "inputs."+tc.key)
		})
	}
}

func TestValidateInputs_IntegerBounds(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	_, issues := schema.ValidateInputs(tmpl, map[string]any{"postgres_port": 70000})
	assert.Contains(t, inputIssuePaths(issues), "inputs.postgres_port")

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"postgres_port": 0})
	assert.Contains(t, inputIssuePaths(issues), "inputs.postgres_port")
}

func TestValidateInputs_Pattern(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	_, issues := schema.ValidateInputs(tmpl, map[string]any{"public_url": "http://insecure.example.com"})
	require.Contains(t, inputIssuePaths(issues), "inputs.public_url")
}

func TestValidateInputs_Enum(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	_, issues := schema.ValidateInputs(tmpl, map[string]any{"maxmemory_policy": "delete-everything"})
	require.Len(t, issues, 1)
	assert.Equal(t, "inputs.maxmemory_policy", issues[0].Path)
	assert.Contains(t, issues[0].Message, "must be one of")

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"maxmemory_policy": "allkeys-lru"})
	assert.Empty(t, issues)
}

// required: true wins over a declared default: the user must still provide
// a value.
func TestValidateInputs_RequiredWithDefault(t *testing.T) {
	t.Parallel()
	tmpl := &schema.Template{
		Inputs: map[string]schema.Input{
			"region": {Type: "string", Default: "eu-central", Required: true},
		},
	}
	_, issues := schema.ValidateInputs(tmpl, nil)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "required input missing")
}

func TestValidateInputs_BooleanAndLengths(t *testing.T) {
	t.Parallel()
	three, ten := 3, 10
	tmpl := &schema.Template{
		Inputs: map[string]schema.Input{
			"debug": {Type: "boolean", Default: false},
			"name":  {Type: "string", MinLength: &three, MaxLength: &ten},
		},
	}
	values, issues := schema.ValidateInputs(tmpl, map[string]any{"name": "abcd"})
	require.Empty(t, issues)
	assert.Equal(t, false, values["debug"])
	assert.Equal(t, "abcd", values["name"])

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"name": "ab"})
	assert.Contains(t, inputIssuePaths(issues), "inputs.name")

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"name": "abcdefghijk"})
	assert.Contains(t, inputIssuePaths(issues), "inputs.name")

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"debug": "yes"})
	assert.Contains(t, inputIssuePaths(issues), "inputs.debug")
}

func TestValidateInputs_IntegerEnum(t *testing.T) {
	t.Parallel()
	tmpl := &schema.Template{
		Inputs: map[string]schema.Input{
			// Decoded templates carry enum numbers as float64; the check
			// must still match int64 values against them.
			"replicas": {Type: "integer", Enum: []any{float64(1), float64(3), float64(5)}},
		},
	}
	values, issues := schema.ValidateInputs(tmpl, map[string]any{"replicas": 3})
	require.Empty(t, issues)
	assert.Equal(t, int64(3), values["replicas"])

	_, issues = schema.ValidateInputs(tmpl, map[string]any{"replicas": 2})
	assert.Contains(t, inputIssuePaths(issues), "inputs.replicas")
}

// Optional inputs without a default stay absent from the result so the
// renderer can tell "not set" apart from "empty".
func TestValidateInputs_OptionalAbsent(t *testing.T) {
	t.Parallel()
	tmpl := &schema.Template{
		Inputs: map[string]schema.Input{
			"comment": {Type: "string"},
		},
	}
	values, issues := schema.ValidateInputs(tmpl, nil)
	assert.Empty(t, issues)
	_, ok := values["comment"]
	assert.False(t, ok)
}
