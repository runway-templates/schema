package schema_test

import (
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refErrors returns only SeverityError issues, which is what the reference
// lint emits (license/minPlan checks emit warnings).
func refErrors(t *schema.Template) []schema.Issue {
	var out []schema.Issue
	for _, is := range schema.Lint(t) {
		if is.Severity == schema.SeverityError {
			out = append(out, is)
		}
	}
	return out
}

func errorPaths(t *schema.Template) []string {
	var out []string
	for _, is := range refErrors(t) {
		out = append(out, is.Path)
	}
	return out
}

// Every checked-in template must be free of reference errors.
func TestLintRefs_ValidFixturesClean(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"mariadb.yaml", "meilisearch.yaml", "mongodb.yaml", "n8n.yaml", "no-plan.yaml", "valkey.yaml"} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, refErrors(loadFixture(t, f)))
		})
	}
}

func TestLintRefs_UnknownInput(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Env["EXTRA"] = schema.EnvValue{Value: "${{ inputs.nope }}"}
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Equal(t, "services[0].env.EXTRA", issues[0].Path)
	assert.Contains(t, issues[0].Message, `unknown input "nope"`)
}

func TestLintRefs_UnknownEnvKey(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Outputs["BROKEN"] = "${{ env.NO_SUCH_KEY }}"
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Equal(t, "services[0].outputs.BROKEN", issues[0].Path)
}

func TestLintRefs_EnvSelfReference(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Env["LOOP"] = schema.EnvValue{Value: "${{ env.LOOP }}"}
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "references itself")
}

func TestLintRefs_MalformedExpression(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Env["BAD"] = schema.EnvValue{Value: "${{ env.PORT"}
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "invalid interpolation")
}

// A second service may consume valkey's outputs; the reference resolves
// and produces no error.
func TestLintRefs_CrossServiceOutput(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services = append(tmpl.Services, schema.Service{
		Name:  "app",
		Image: "nginx:1.27",
		Env: map[string]schema.EnvValue{
			"REDIS_DSN": {Value: "${{ services.valkey.outputs.DSN }}"},
		},
	})
	assert.Empty(t, refErrors(tmpl))
}

func TestLintRefs_OutputRefErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		value   string
		message string
	}{
		"unknown service": {"${{ services.ghost.outputs.DSN }}", `unknown service "ghost"`},
		"unknown output":  {"${{ services.valkey.outputs.NOPE }}", `no output "NOPE"`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tmpl := loadFixture(t, "valkey.yaml")
			tmpl.Services = append(tmpl.Services, schema.Service{
				Name:  "app",
				Image: "nginx:1.27",
				Env:   map[string]schema.EnvValue{"REF": {Value: tc.value}},
			})
			issues := refErrors(tmpl)
			require.Len(t, issues, 1)
			assert.Equal(t, "services[1].env.REF", issues[0].Path)
			assert.Contains(t, issues[0].Message, tc.message)
		})
	}
}

func TestLintRefs_OwnOutputReference(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Env["SELF"] = schema.EnvValue{Value: "${{ services.valkey.outputs.DSN }}"}
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "its own outputs")
}

func TestLintRefs_CyclicOutputs(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Env["PEER"] = schema.EnvValue{Value: "${{ services.app.outputs.URL }}"}
	tmpl.Services = append(tmpl.Services, schema.Service{
		Name:    "app",
		Image:   "nginx:1.27",
		Env:     map[string]schema.EnvValue{"REDIS_DSN": {Value: "${{ services.valkey.outputs.DSN }}"}},
		Outputs: map[string]string{"URL": "http://app"},
	})
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Equal(t, "services", issues[0].Path)
	assert.Contains(t, issues[0].Message, "cyclic output references")
}

// valkey.yaml sets settings.route: false, so runway.url must be rejected
// there, while a service with routing left on may use it.
func TestLintRefs_RunwayURLRequiresRoute(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Outputs["PUBLIC"] = "${{ runway.url }}"
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "settings.route is false")

	routed := loadFixture(t, "n8n.yaml")
	routed.Services[0].Outputs["PUBLIC"] = "${{ runway.url }}"
	assert.Empty(t, refErrors(routed))
}

func TestLintRefs_UnknownRunwayField(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Outputs["REGION"] = "${{ runway.region }}"
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, `unknown runway field "region"`)
}

func TestLintRefs_SecretBounds(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"15", "129"} {
		tmpl := loadFixture(t, "valkey.yaml")
		tmpl.Services[0].Env["KEY"] = schema.EnvValue{Value: "${{ runway.secret(" + n + ") }}"}
		issues := refErrors(tmpl)
		require.Len(t, issues, 1, n)
		assert.Contains(t, issues[0].Message, "out of range")
	}
}

// Secrets belong in env values only; a secret in command args would leak
// into the pod spec.
func TestLintRefs_SecretNotAllowedInCommand(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services[0].Command = append(tmpl.Services[0].Command, "${{ runway.secret(32) }}")
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "not allowed in this context")
}

func TestLintRefs_DuplicateServiceNames(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "valkey.yaml")
	tmpl.Services = append(tmpl.Services, schema.Service{Name: "valkey", Image: "nginx:1.27"})
	issues := refErrors(tmpl)
	require.NotEmpty(t, issues)
	assert.Equal(t, "services[1].name", issues[0].Path)
	assert.Contains(t, issues[0].Message, "duplicate service name")
}

// Init and worker containers see the parent service's env keys.
func TestLintRefs_ContainerEnvScope(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")
	require.NotEmpty(t, tmpl.Services[0].Workers)
	tmpl.Services[0].Workers[0].Env = map[string]schema.EnvValue{
		"FROM_PARENT": {Value: "${{ env.PORT }}"},
	}
	assert.Empty(t, refErrors(tmpl))

	tmpl.Services[0].Workers[0].Env["BROKEN"] = schema.EnvValue{Value: "${{ env.NOT_THERE }}"}
	issues := refErrors(tmpl)
	require.Len(t, issues, 1)
	assert.Equal(t, "services[0].workers[0].env.BROKEN", issues[0].Path)
}
