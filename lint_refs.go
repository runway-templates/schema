package schema

import (
	"fmt"
	"slices"
	"strings"
)

// runwayFields are the platform variables templates may reference as
// ${{ runway.<field> }}. See docs/interpolation/runway.md.
var runwayFields = map[string]struct{}{
	"stack":            {},
	"fqdn":             {},
	"url":              {},
	"internal_address": {},
}

// Secret length bounds for ${{ runway.secret(n) }}.
const (
	secretMinLen = 16
	secretMaxLen = 128
)

// Allowed expression kinds per context, mirroring the allowed-contexts
// table in docs/interpolation/interpolation.md.
var (
	allowImage       = kindSet(ExprInput)
	allowCommand     = kindSet(ExprInput, ExprOutput, ExprEnv, ExprRunway)
	allowEnv         = kindSet(ExprInput, ExprOutput, ExprEnv, ExprRunway, ExprSecret)
	allowOutputs     = kindSet(ExprInput, ExprEnv, ExprRunway)
	allowHealthcheck = kindSet(ExprInput)
)

func kindSet(kinds ...ExprKind) map[ExprKind]bool {
	s := make(map[ExprKind]bool, len(kinds))
	for _, k := range kinds {
		s[k] = true
	}
	return s
}

// lintRefs implements the reference rules from docs/interpolation/interpolation.md:
// every expression must parse, resolve against the template, and appear
// in an allowed context; cross-service output references must form a DAG.
// All issues are errors — a template that fails them cannot deploy.
func lintRefs(t *Template) []Issue {
	l := &refLinter{
		tmpl:     t,
		services: make(map[string]*Service, len(t.Services)),
		deps:     make(map[string][]string),
	}
	for i := range t.Services {
		svc := &t.Services[i]
		if _, dup := l.services[svc.Name]; dup {
			l.errf(fmt.Sprintf("services[%d].name", i), "duplicate service name %q", svc.Name)
			continue
		}
		l.services[svc.Name] = svc
	}
	for i := range t.Services {
		l.lintService(i, &t.Services[i])
	}
	l.lintCycles()
	return l.issues
}

type refLinter struct {
	tmpl     *Template
	services map[string]*Service
	// deps records valid cross-service output references, per service.
	deps   map[string][]string
	cur    *Service
	issues []Issue
}

// refScope is what a value may reference: the allowed expression kinds,
// the visible env keys, and selfKey, the env key being defined when the
// value is an env value.
type refScope struct {
	allowed map[ExprKind]bool
	env     map[string]bool
	selfKey string
}

// container is the part of an init or worker container that can hold
// references.
type container struct {
	image   string
	command []string
	args    []string
	env     map[string]EnvValue
}

func (l *refLinter) errf(path, format string, args ...any) {
	l.issues = append(l.issues, Issue{
		Path:     path,
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityError,
	})
}

func (l *refLinter) lintService(idx int, svc *Service) {
	l.cur = svc
	base := fmt.Sprintf("services[%d]", idx)
	mainEnv := envKeys(svc.Env, nil)

	commandScope := refScope{allowed: allowCommand, env: mainEnv}
	healthcheckScope := refScope{allowed: allowHealthcheck}

	l.check(base+".image", svc.Image, refScope{allowed: allowImage})
	l.checkList(base+".command", svc.Command, commandScope)
	l.checkList(base+".args", svc.Args, commandScope)
	l.checkEnv(base+".env", svc.Env, mainEnv)

	for key, value := range svc.Outputs {
		l.check(base+".outputs."+key, value, refScope{allowed: allowOutputs, env: mainEnv})
	}
	if hc := svc.Healthcheck; hc != nil {
		l.check(base+".healthcheck.path", hc.Path, healthcheckScope)
		l.checkList(base+".healthcheck.command", hc.Command, healthcheckScope)
	}
	// init and worker containers run in the same pod, so their names must
	// be unique across both lists
	names := make(map[string]bool, len(svc.Init)+len(svc.Workers))
	for i, ic := range svc.Init {
		path := fmt.Sprintf("%s.init[%d]", base, i)
		l.lintContainer(path, container{image: ic.Image, command: ic.Command, args: ic.Args, env: ic.Env}, svc.Env)
		if names[ic.Name] {
			l.errf(path+".name", "duplicate container name %q", ic.Name)
		}
		names[ic.Name] = true
	}
	for i, wc := range svc.Workers {
		path := fmt.Sprintf("%s.workers[%d]", base, i)
		l.lintContainer(path, container{image: wc.Image, command: wc.Command, args: wc.Args, env: wc.Env}, svc.Env)
		if names[wc.Name] {
			l.errf(path+".name", "duplicate container name %q", wc.Name)
		}
		names[wc.Name] = true
	}
}

// lintContainer checks an init or worker container. Its env scope is the
// parent service's env merged with its own overrides.
func (l *refLinter) lintContainer(base string, c container, parentEnv map[string]EnvValue) {
	env := envKeys(parentEnv, c.env)
	commandScope := refScope{allowed: allowCommand, env: env}
	l.check(base+".image", c.image, refScope{allowed: allowImage})
	l.checkList(base+".command", c.command, commandScope)
	l.checkList(base+".args", c.args, commandScope)
	l.checkEnv(base+".env", c.env, env)
}

func (l *refLinter) checkList(base string, values []string, scope refScope) {
	for i, v := range values {
		l.check(fmt.Sprintf("%s[%d]", base, i), v, scope)
	}
}

func (l *refLinter) checkEnv(base string, env map[string]EnvValue, visible map[string]bool) {
	for key, value := range env {
		l.check(base+"."+key, value.Value, refScope{allowed: allowEnv, env: visible, selfKey: key})
	}
}

// check parses s and validates each expression: allowed in this context,
// and resolvable.
func (l *refLinter) check(path, s string, scope refScope) {
	exprs, err := Expressions(s)
	if err != nil {
		l.errf(path, "invalid interpolation: %v", err)
		return
	}
	for _, e := range exprs {
		if !scope.allowed[e.Kind] {
			l.errf(path, "%s is not allowed in this context", e)
			continue
		}
		l.checkExpr(path, e, scope)
	}
}

func (l *refLinter) checkExpr(path string, e Expr, scope refScope) {
	switch e.Kind {
	case ExprInput:
		if _, ok := l.tmpl.Inputs[e.Name]; !ok {
			l.errf(path, "unknown input %q", e.Name)
		}
	case ExprOutput:
		l.checkOutputRef(path, e)
	case ExprEnv:
		if e.Name == scope.selfKey {
			l.errf(path, "env %q references itself", e.Name)
		} else if !scope.env[e.Name] {
			l.errf(path, "unknown env key %q in this service", e.Name)
		}
	case ExprRunway:
		l.checkRunwayField(path, e)
	case ExprSecret:
		if e.N < secretMinLen || e.N > secretMaxLen {
			l.errf(path, "secret length %d out of range [%d, %d]", e.N, secretMinLen, secretMaxLen)
		}
	default:
		l.errf(path, "unknown expression kind %v", e.Kind)
	}
}

func (l *refLinter) checkOutputRef(path string, e Expr) {
	if e.Service == l.cur.Name {
		l.errf(path, "service %q references its own outputs; use env.* instead", e.Service)
		return
	}
	target, ok := l.services[e.Service]
	if !ok {
		l.errf(path, "unknown service %q", e.Service)
		return
	}
	if _, ok := target.Outputs[e.Output]; !ok {
		l.errf(path, "service %q has no output %q", e.Service, e.Output)
		return
	}
	if !slices.Contains(l.deps[l.cur.Name], e.Service) {
		l.deps[l.cur.Name] = append(l.deps[l.cur.Name], e.Service)
	}
}

func (l *refLinter) checkRunwayField(path string, e Expr) {
	if _, ok := runwayFields[e.Name]; !ok {
		l.errf(path, "unknown runway field %q", e.Name)
		return
	}
	if e.Name == "url" && l.cur.Settings != nil && l.cur.Settings.Route != nil && !*l.cur.Settings.Route {
		l.errf(path, "runway.url is invalid here: settings.route is false for service %q", l.cur.Name)
	}
	if e.Name == "internal_address" {
		if _, ok := l.cur.Env["PORT"]; !ok {
			l.errf(path, "runway.internal_address needs env.PORT in service %q", l.cur.Name)
		}
	}
}

// lintCycles reports the first cycle in the cross-service reference graph
// collected by checkOutputRef. The deployer renders services in dependency
// order, so a cycle makes the template undeployable.
func (l *refLinter) lintCycles() {
	const (
		inProgress = 1
		done       = 2
	)
	state := make(map[string]int)
	var stack []string

	var visit func(name string) bool
	visit = func(name string) bool {
		switch state[name] {
		case inProgress:
			cycle := append(slices.Clone(stack[slices.Index(stack, name):]), name)
			l.errf("services", "cyclic output references: %s", strings.Join(cycle, " -> "))
			return true
		case done:
			return false
		}
		state[name] = inProgress
		stack = append(stack, name)
		for _, dep := range l.deps[name] {
			if visit(dep) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = done
		return false
	}
	for i := range l.tmpl.Services {
		if visit(l.tmpl.Services[i].Name) {
			return
		}
	}
}

// envKeys returns the set of env keys visible in a container: the parent
// env plus container-level overrides.
func envKeys(parent, overrides map[string]EnvValue) map[string]bool {
	keys := make(map[string]bool, len(parent)+len(overrides))
	for k := range parent {
		keys[k] = true
	}
	for k := range overrides {
		keys[k] = true
	}
	return keys
}
