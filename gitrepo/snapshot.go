// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckoutResult is a prepared working tree at a resolved commit.
type CheckoutResult struct {
	Path              string
	RequestedRef      string
	ResolvedCommitSHA string
	CloneURL          string
	Managed           bool
}

// SnapshotOptions configures a one-shot or managed clone checkout.
type SnapshotOptions struct {
	RemoteURL          string
	LocalPath          string // existing checkout
	Ref                string
	CacheRoot          string // parent for managed clones
	SourceName         string
	AcquisitionMode    string // snapshot|managed_clone|local_checkout
	CloneDepth         int
	PartialCloneFilter string
	SparsePaths        []string
	WorkingTreeMode    string // HEAD|working_tree
	Runner             *Runner
}

// PrepareCheckout acquires a repository tree for ingestion.
func PrepareCheckout(ctx context.Context, opt SnapshotOptions) (*CheckoutResult, error) {
	r := opt.Runner
	if r == nil {
		r = &Runner{}
	}
	mode := opt.AcquisitionMode
	if mode == "" {
		if opt.LocalPath != "" {
			mode = "local_checkout"
		} else {
			mode = "snapshot"
		}
	}
	switch mode {
	case "local_checkout":
		return prepareLocal(ctx, r, opt)
	case "managed_clone", "snapshot":
		return prepareClone(ctx, r, opt, mode == "managed_clone")
	default:
		return nil, fmt.Errorf("unknown acquisition mode %q", mode)
	}
}

func prepareLocal(ctx context.Context, r *Runner, opt SnapshotOptions) (*CheckoutResult, error) {
	if opt.LocalPath == "" {
		return nil, fmt.Errorf("localPath is required for local_checkout")
	}
	abs, err := filepath.Abs(opt.LocalPath)
	if err != nil {
		return nil, err
	}
	ref := opt.Ref
	if ref == "" {
		ref = "HEAD"
	}
	mode := opt.WorkingTreeMode
	if mode == "" {
		mode = "HEAD"
	}
	sha, err := r.Run(ctx, abs, "rev-parse", ref)
	if err != nil {
		return nil, err
	}
	if mode == "HEAD" {
		// Detach to exact commit in a temp worktree for reproducibility when dirty?
		// For HEAD mode index committed tree via git archive / worktree.
		tmp, err := os.MkdirTemp("", "implcache-git-head-*")
		if err != nil {
			return nil, err
		}
		if _, err := r.Run(ctx, abs, "worktree", "add", "--detach", tmp, sha); err != nil {
			_ = os.RemoveAll(tmp)
			// Fallback: use working directory but warn via checkout of files from sha
			return &CheckoutResult{Path: abs, RequestedRef: ref, ResolvedCommitSHA: sha, Managed: false}, nil
		}
		if err := applySparse(ctx, r, tmp, opt.SparsePaths); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, fmt.Errorf("sparse-checkout on local HEAD worktree: %w", err)
		}
		return &CheckoutResult{Path: tmp, RequestedRef: ref, ResolvedCommitSHA: sha, Managed: false}, nil
	}
	// working_tree: index filesystem as-is
	return &CheckoutResult{Path: abs, RequestedRef: ref, ResolvedCommitSHA: sha, Managed: false}, nil
}

func prepareClone(ctx context.Context, r *Runner, opt SnapshotOptions, managed bool) (*CheckoutResult, error) {
	n, err := NormalizeRemoteURL(opt.RemoteURL)
	if err != nil {
		return nil, err
	}
	ref := opt.Ref
	if ref == "" {
		insp, err := InspectRepo(ctx, InspectOptions{RemoteURL: n.CloneURL, Runner: r})
		if err != nil {
			return nil, err
		}
		if insp.DefaultBranch != "" {
			ref = insp.DefaultBranch
		} else {
			ref = "HEAD"
		}
	}
	depth := opt.CloneDepth
	if depth <= 0 {
		depth = 1
	}
	cacheRoot := opt.CacheRoot
	if cacheRoot == "" {
		cacheRoot = filepath.Join(os.TempDir(), "implcache-repos")
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return nil, err
	}
	name := opt.SourceName
	if name == "" {
		name = n.Repository
		if name == "" {
			name = "repo"
		}
	}
	dest := filepath.Join(cacheRoot, sanitizeName(name))
	if !managed {
		dest = filepath.Join(cacheRoot, sanitizeName(name)+"-"+sanitizeName(ref))
		_ = os.RemoveAll(dest)
	}

	if managed {
		if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
			// fetch + checkout
			if _, err := r.Run(ctx, dest, "remote", "set-url", "origin", n.CloneURL); err != nil {
				return nil, err
			}
			fetchArgs := []string{"fetch", "--depth", fmt.Sprintf("%d", depth), "origin", "+" + ref + ":refs/remotes/origin/" + ref}
			if _, err := r.Run(ctx, dest, fetchArgs...); err != nil {
				// try without depth for tags
				if _, err2 := r.Run(ctx, dest, "fetch", "origin", "tag", ref, "--force"); err2 != nil {
					if _, err3 := r.Run(ctx, dest, "fetch", "--depth", fmt.Sprintf("%d", depth), "origin", ref); err3 != nil {
						if _, err4 := r.Run(ctx, dest, "fetch", "origin", ref); err4 != nil {
							return nil, err
						}
					}
				}
			}
			sha, err := resolveSHA(ctx, r, dest, ref)
			if err != nil {
				return nil, err
			}
			if err := applySparse(ctx, r, dest, opt.SparsePaths); err != nil {
				return nil, err
			}
			if _, err := r.Run(ctx, dest, "checkout", "--force", sha); err != nil {
				return nil, err
			}
			return &CheckoutResult{
				Path: dest, RequestedRef: ref, ResolvedCommitSHA: sha,
				CloneURL: n.CloneURL, Managed: true,
			}, nil
		}
	}

	args := []string{"clone", "--no-checkout"}
	if opt.PartialCloneFilter != "" {
		args = append(args, "--filter="+opt.PartialCloneFilter)
	}
	args = append(args, "--depth", fmt.Sprintf("%d", depth))
	// Prefer single-branch when ref looks like a branch name
	if !looksLikeSHA(ref) {
		args = append(args, "--branch", ref, "--single-branch")
	}
	args = append(args, n.CloneURL, dest)
	if _, err := r.Run(ctx, "", args...); err != nil {
		// Retry without --branch (tag/commit)
		_ = os.RemoveAll(dest)
		args = []string{"clone", "--no-checkout", "--depth", fmt.Sprintf("%d", depth), n.CloneURL, dest}
		if opt.PartialCloneFilter != "" {
			args = []string{"clone", "--no-checkout", "--filter=" + opt.PartialCloneFilter, "--depth", fmt.Sprintf("%d", depth), n.CloneURL, dest}
		}
		if _, err2 := r.Run(ctx, "", args...); err2 != nil {
			// Partial filter unsupported → fallback
			if opt.PartialCloneFilter != "" {
				_ = os.RemoveAll(dest)
				args = []string{"clone", "--no-checkout", "--depth", fmt.Sprintf("%d", depth), n.CloneURL, dest}
				if _, err3 := r.Run(ctx, "", args...); err3 != nil {
					return nil, err2
				}
			} else {
				return nil, err
			}
		}
		if _, err := r.Run(ctx, dest, "fetch", "--depth", fmt.Sprintf("%d", depth), "origin", ref); err != nil {
			_, _ = r.Run(ctx, dest, "fetch", "origin", "tag", ref, "--force")
		}
	}
	sha, err := resolveSHA(ctx, r, dest, ref)
	if err != nil {
		return nil, err
	}
	if err := applySparse(ctx, r, dest, opt.SparsePaths); err != nil {
		return nil, err
	}
	if _, err := r.Run(ctx, dest, "checkout", "--force", sha); err != nil {
		return nil, err
	}
	return &CheckoutResult{
		Path: dest, RequestedRef: ref, ResolvedCommitSHA: sha,
		CloneURL: n.CloneURL, Managed: managed,
	}, nil
}

func resolveSHA(ctx context.Context, r *Runner, dir, ref string) (string, error) {
	// Prefer remote-tracking / FETCH_HEAD after fetch so shallow updates are seen.
	candidates := []string{
		"origin/" + ref,
		"refs/remotes/origin/" + ref,
		"FETCH_HEAD",
		"refs/tags/" + ref,
		"refs/tags/" + ref + "^{}",
		"refs/heads/" + ref,
		ref,
	}
	for _, c := range candidates {
		sha, err := r.Run(ctx, dir, "rev-parse", c)
		if err == nil && looksLikeSHA(sha) {
			return sha, nil
		}
	}
	return "", fmt.Errorf("could not resolve ref %q", ref)
}

func applySparse(ctx context.Context, r *Runner, dir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if _, err := r.Run(ctx, dir, "sparse-checkout", "init", "--cone"); err != nil {
		return err
	}
	args := append([]string{"sparse-checkout", "set"}, paths...)
	_, err := r.Run(ctx, dir, args...)
	return err
}

func looksLikeSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func sanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, s)
	return strings.Trim(s, "-")
}

// DiffNameStatus returns added/modified/deleted/renamed paths between two commits.
func DiffNameStatus(ctx context.Context, r *Runner, dir, oldSHA, newSHA string) (added, modified, deleted []string, err error) {
	if r == nil {
		r = &Runner{}
	}
	out, err := r.Run(ctx, dir, "diff", "--name-status", oldSHA, newSHA)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		switch {
		case status == "A" || strings.HasPrefix(status, "A"):
			added = append(added, fields[len(fields)-1])
		case status == "M" || strings.HasPrefix(status, "M") || strings.HasPrefix(status, "T"):
			modified = append(modified, fields[len(fields)-1])
		case status == "D" || strings.HasPrefix(status, "D"):
			deleted = append(deleted, fields[1])
		case strings.HasPrefix(status, "R"):
			// rename: old new → delete old, add new
			if len(fields) >= 3 {
				deleted = append(deleted, fields[1])
				added = append(added, fields[2])
			}
		}
	}
	return added, modified, deleted, nil
}
