// Package release checks a template repository and tags a new version.
package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/runway-templates/schema"
	"golang.org/x/mod/semver"
)

// TemplateFile is the template file expected at the repository root.
const TemplateFile = "template.yaml"

const remote = "origin"

// Errors returned by Check.
var (
	ErrNoTemplate        = errors.New("no " + TemplateFile + " in repository root")
	ErrTemplateInvalid   = errors.New("template is invalid")
	ErrInvalidVersion    = errors.New("metadata.version is not a valid semver version")
	ErrDirty             = errors.New("working tree is not clean")
	ErrNotDefaultBranch  = errors.New("not on the default branch")
	ErrOutOfSync         = errors.New("local branch is not in sync with remote")
	ErrHeadTagged        = errors.New("HEAD is already tagged")
	ErrNeedsBump         = errors.New("version is already tagged, bump metadata.version")
	ErrVersionNotGreater = errors.New("version is not greater than the latest tag")
)

// Plan is the result of a successful Check.
type Plan struct {
	Dir         string
	Version     string
	Tag         string
	PreviousTag string
	Branch      string
	Changelog   []string
	Warnings    []schema.Issue
}

// Check runs all pre-release checks in the repository at dir. It fetches
// from origin but does not change the working tree.
func Check(ctx context.Context, dir string) (*Plan, error) {
	g := gitRunner{dir: dir}
	root, err := g.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	g.dir = root

	p := &Plan{Dir: root}
	if err := p.checkTemplate(); err != nil {
		return nil, err
	}
	if err := p.checkRepo(ctx, g); err != nil {
		return nil, err
	}
	if err := p.checkTags(ctx, g); err != nil {
		return nil, err
	}
	p.Changelog, err = changelog(ctx, g, p.PreviousTag)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Plan) checkTemplate() error {
	path := filepath.Join(p.Dir, TemplateFile)
	if _, err := os.Stat(path); err != nil {
		return ErrNoTemplate
	}
	if err := schema.ValidateFile(path); err != nil {
		return fmt.Errorf("%w: %w", ErrTemplateInvalid, err)
	}
	issues, err := schema.LintFile(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemplateInvalid, err)
	}
	for _, issue := range issues {
		if issue.Severity == schema.SeverityError {
			return fmt.Errorf("%w: %s", ErrTemplateInvalid, issue)
		}
	}
	p.Warnings = issues

	t, err := schema.DecodeFile(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTemplateInvalid, err)
	}
	p.Version = t.Metadata.Version
	p.Tag = "v" + p.Version
	if !semver.IsValid(p.Tag) || semver.Canonical(p.Tag) != p.Tag {
		return fmt.Errorf("%w: %q", ErrInvalidVersion, p.Version)
	}
	return nil
}

func (p *Plan) checkRepo(ctx context.Context, g gitRunner) error {
	status, err := g.run(ctx, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("%w:\n%s", ErrDirty, status)
	}

	if _, err := g.run(ctx, "fetch", "--tags", "--force", remote); err != nil {
		return err
	}

	head, err := g.run(ctx, "symbolic-ref", "--short", "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return fmt.Errorf("unknown default branch, try 'git remote set-head %s --auto': %w", remote, err)
	}
	p.Branch = strings.TrimPrefix(head, remote+"/")

	current, err := g.run(ctx, "branch", "--show-current")
	if err != nil {
		return err
	}
	if current != p.Branch {
		return fmt.Errorf("%w: on %q, want %q", ErrNotDefaultBranch, current, p.Branch)
	}

	counts, err := g.run(ctx, "rev-list", "--left-right", "--count", "HEAD..."+head)
	if err != nil {
		return err
	}
	var ahead, behind int
	if _, err := fmt.Sscanf(counts, "%d %d", &ahead, &behind); err != nil {
		return fmt.Errorf("parse rev-list output %q: %w", counts, err)
	}
	if ahead != 0 || behind != 0 {
		return fmt.Errorf("%w: %d ahead, %d behind %s", ErrOutOfSync, ahead, behind, head)
	}
	return nil
}

func (p *Plan) checkTags(ctx context.Context, g gitRunner) error {
	headTags, err := g.lines(ctx, "tag", "--points-at", "HEAD", "--list", "v*")
	if err != nil {
		return err
	}
	if len(headTags) > 0 {
		return fmt.Errorf("%w: %s", ErrHeadTagged, strings.Join(headTags, ", "))
	}

	tags, err := versionTags(ctx, g)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil
	}
	p.PreviousTag = tags[len(tags)-1]

	if slices.Contains(tags, p.Tag) {
		next := strings.Join(nextVersions(p.PreviousTag), ", ")
		return fmt.Errorf("%w: %s exists, next could be %s", ErrNeedsBump, p.Tag, next)
	}
	if semver.Compare(p.Tag, p.PreviousTag) <= 0 {
		return fmt.Errorf("%w: %s <= %s", ErrVersionNotGreater, p.Tag, p.PreviousTag)
	}
	return nil
}

// versionTags returns all valid semver tags, local and remote, sorted
// ascending.
func versionTags(ctx context.Context, g gitRunner) ([]string, error) {
	local, err := g.lines(ctx, "tag", "--list", "v*")
	if err != nil {
		return nil, err
	}
	refs, err := g.lines(ctx, "ls-remote", "--tags", "--refs", remote, "v*")
	if err != nil {
		return nil, err
	}

	tags := local
	for _, ref := range refs {
		_, name, ok := strings.Cut(ref, "refs/tags/")
		if ok {
			tags = append(tags, name)
		}
	}
	tags = slices.DeleteFunc(tags, func(t string) bool { return !semver.IsValid(t) })
	semver.Sort(tags)
	return slices.Compact(tags), nil
}

func nextVersions(tag string) []string {
	var major, minor, patch int
	if _, err := fmt.Sscanf(semver.Canonical(tag), "v%d.%d.%d", &major, &minor, &patch); err != nil {
		return nil
	}
	return []string{
		fmt.Sprintf("%d.%d.%d", major, minor, patch+1),
		fmt.Sprintf("%d.%d.0", major, minor+1),
		fmt.Sprintf("%d.0.0", major+1),
	}
}

func changelog(ctx context.Context, g gitRunner, previousTag string) ([]string, error) {
	rev := "HEAD"
	if previousTag != "" {
		rev = previousTag + "..HEAD"
	}
	return g.lines(ctx, "log", "--no-merges", "--format=%s", rev)
}

// Message returns the annotated tag message for the plan.
func (p *Plan) Message() string {
	var b strings.Builder
	fmt.Fprintln(&b, p.Tag)
	if len(p.Changelog) > 0 {
		fmt.Fprintln(&b)
	}
	for _, line := range p.Changelog {
		fmt.Fprintln(&b, "-", line)
	}
	return b.String()
}

// Publish creates a signed, annotated tag for the plan and pushes it to
// origin. The local tag is removed if the push fails.
func Publish(ctx context.Context, p *Plan) error {
	g := gitRunner{dir: p.Dir}
	if _, err := g.run(ctx, "tag", "-a", "-s", "-m", p.Message(), p.Tag); err != nil {
		return err
	}
	if _, err := g.run(ctx, "push", remote, "refs/tags/"+p.Tag); err != nil {
		if _, delErr := g.run(ctx, "tag", "-d", p.Tag); delErr != nil {
			return errors.Join(err, delErr)
		}
		return err
	}
	return nil
}
