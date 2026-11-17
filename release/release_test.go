package release_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runway-templates/schema/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const templateFmt = `apiVersion: schema.runway.horse/templates/v1alpha1
kind: Template
metadata:
  name: demo
  version: "%s"
  displayName: Demo
  description: Demo template.
  category: database
  license: MIT
services:
  - name: web
    image: nginx:1.27
    minPlan: free-launch
    env:
      PORT: "8080"
`

type repo struct {
	t      *testing.T
	dir    string
	origin string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	base := t.TempDir()

	key := filepath.Join(base, "key")
	out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput()
	require.NoError(t, err, string(out))

	gitconfig := filepath.Join(base, "gitconfig")
	cfg := fmt.Sprintf(`[user]
	name = Test
	email = test@example.org
	signingkey = %s.pub
[gpg]
	format = ssh
[init]
	defaultBranch = main
`, key)
	require.NoError(t, os.WriteFile(gitconfig, []byte(cfg), 0o600))
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	r := &repo{t: t, dir: filepath.Join(base, "work"), origin: filepath.Join(base, "origin.git")}
	r.git(base, "init", "--bare", r.origin)
	r.git(base, "clone", r.origin, r.dir)
	r.commit("feat: initial", "0.1.0")
	r.git(r.dir, "push", "origin", "main")
	r.git(r.dir, "remote", "set-head", "origin", "--auto")
	return r
}

func (r *repo) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(r.t, err, string(out))
	return strings.TrimSpace(string(out))
}

func (r *repo) commit(msg, version string) {
	r.t.Helper()
	path := filepath.Join(r.dir, release.TemplateFile)
	require.NoError(r.t, os.WriteFile(path, fmt.Appendf(nil, templateFmt, version), 0o600))
	r.git(r.dir, "add", "-A")
	r.git(r.dir, "commit", "--allow-empty", "-m", msg)
}

func (r *repo) push() { r.git(r.dir, "push", "origin", "main") }

func (r *repo) release() {
	r.t.Helper()
	p, err := release.Check(context.Background(), r.dir)
	require.NoError(r.t, err)
	require.NoError(r.t, release.Publish(context.Background(), p))
}

func TestCheck_FirstRelease(t *testing.T) {
	r := newRepo(t)

	p, err := release.Check(context.Background(), r.dir)
	require.NoError(t, err)
	assert.Equal(t, "0.1.0", p.Version)
	assert.Equal(t, "v0.1.0", p.Tag)
	assert.Empty(t, p.PreviousTag)
	assert.Equal(t, "main", p.Branch)
	assert.Equal(t, []string{"feat: initial"}, p.Changelog)
	assert.Equal(t, "v0.1.0\n\n- feat: initial\n", p.Message())
}

func TestPublish_SignedTagPushed(t *testing.T) {
	r := newRepo(t)
	r.release()

	remote := r.git(r.dir, "ls-remote", "--tags", "origin", "v0.1.0")
	assert.Contains(t, remote, "refs/tags/v0.1.0")

	obj := r.git(r.origin, "cat-file", "-p", "v0.1.0")
	assert.Contains(t, obj, "- feat: initial")
	assert.Contains(t, obj, "-----BEGIN SSH SIGNATURE-----")
}

func TestPublish_PushFailureRemovesTag(t *testing.T) {
	r := newRepo(t)
	p, err := release.Check(context.Background(), r.dir)
	require.NoError(t, err)

	r.git(r.dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	require.Error(t, release.Publish(context.Background(), p))
	assert.Empty(t, r.git(r.dir, "tag", "--list"))
}

func TestCheck_ChangelogSincePreviousTag(t *testing.T) {
	r := newRepo(t)
	r.release()
	r.commit("fix: one", "0.1.0")
	r.commit("feat: two", "0.2.0")
	r.push()

	p, err := release.Check(context.Background(), r.dir)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", p.PreviousTag)
	assert.Equal(t, []string{"feat: two", "fix: one"}, p.Changelog)
}

func TestCheck_NeedsBump(t *testing.T) {
	r := newRepo(t)
	r.release()
	r.commit("fix: one", "0.1.0")
	r.push()

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrNeedsBump)
	assert.Contains(t, err.Error(), "0.1.1, 0.2.0, 1.0.0")
}

func TestCheck_TagOnlyOnRemote(t *testing.T) {
	r := newRepo(t)
	r.commit("fix: one", "0.1.0")
	r.push()
	r.git(r.dir, "tag", "v0.1.0", "HEAD~1")
	r.git(r.dir, "push", "origin", "v0.1.0")
	r.git(r.dir, "tag", "-d", "v0.1.0")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrNeedsBump)
}

func TestCheck_HeadTagged(t *testing.T) {
	r := newRepo(t)
	r.release()

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrHeadTagged)
}

func TestCheck_VersionNotGreater(t *testing.T) {
	r := newRepo(t)
	r.commit("feat: two", "0.2.0")
	r.push()
	r.release()
	r.commit("fix: back", "0.1.5")
	r.push()

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrVersionNotGreater)
}

func TestCheck_InvalidVersion(t *testing.T) {
	for _, v := range []string{"1", "1.0", "1.0.0+build"} {
		t.Run(v, func(t *testing.T) {
			r := newRepo(t)
			r.commit("chore: version", v)
			r.push()

			_, err := release.Check(context.Background(), r.dir)
			require.ErrorIs(t, err, release.ErrInvalidVersion)
		})
	}
}

func TestCheck_Dirty(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, "new.txt"), []byte("x"), 0o600))

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrDirty)
}

func TestCheck_NotDefaultBranch(t *testing.T) {
	r := newRepo(t)
	r.git(r.dir, "switch", "-c", "feature")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrNotDefaultBranch)
}

func TestCheck_Ahead(t *testing.T) {
	r := newRepo(t)
	r.commit("feat: unpushed", "0.1.0")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrOutOfSync)
	assert.Contains(t, err.Error(), "1 ahead, 0 behind")
}

func TestCheck_Behind(t *testing.T) {
	r := newRepo(t)
	other := filepath.Join(t.TempDir(), "other")
	r.git(r.dir, "clone", r.origin, other)
	r.git(other, "commit", "--allow-empty", "-m", "feat: elsewhere")
	r.git(other, "push", "origin", "main")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrOutOfSync)
	assert.Contains(t, err.Error(), "0 ahead, 1 behind")
}

func TestCheck_NoTemplate(t *testing.T) {
	r := newRepo(t)
	r.git(r.dir, "rm", "-q", release.TemplateFile)
	r.git(r.dir, "commit", "-m", "chore: remove")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrNoTemplate)
}

func TestCheck_InvalidTemplate(t *testing.T) {
	r := newRepo(t)
	path := filepath.Join(r.dir, release.TemplateFile)
	require.NoError(t, os.WriteFile(path, []byte("kind: Nope\n"), 0o600))
	r.git(r.dir, "commit", "-am", "chore: break")

	_, err := release.Check(context.Background(), r.dir)
	require.ErrorIs(t, err, release.ErrTemplateInvalid)
}

func TestCheck_FromSubdirectory(t *testing.T) {
	r := newRepo(t)
	sub := filepath.Join(r.dir, "docs")
	require.NoError(t, os.Mkdir(sub, 0o750))

	p, err := release.Check(context.Background(), sub)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", p.Tag)
}
