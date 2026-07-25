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

// InspectReport describes a repository without ingesting.
type InspectReport struct {
	RemoteURL         string   `json:"remoteUrl,omitempty"`
	LocalPath         string   `json:"localPath,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	Owner             string   `json:"owner,omitempty"`
	Repository        string   `json:"repository,omitempty"`
	DefaultBranch     string   `json:"defaultBranch,omitempty"`
	RequestedRef      string   `json:"requestedRef,omitempty"`
	ResolvedRefType   string   `json:"resolvedRefType,omitempty"`
	ResolvedCommitSHA string   `json:"resolvedCommitSha,omitempty"`
	Accessible        bool     `json:"accessible"`
	AuthRequired      bool     `json:"authRequired,omitempty"`
	CurrentBranch     string   `json:"currentBranch,omitempty"`
	WorkingTreeDirty  bool     `json:"workingTreeDirty,omitempty"`
	ModifiedFiles     int      `json:"modifiedFiles,omitempty"`
	UntrackedFiles    int      `json:"untrackedFiles,omitempty"`
	SubmoduleCount    int      `json:"submoduleCount,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
	CloneURL          string   `json:"cloneUrl,omitempty"`
}

// InspectOptions configures inspection.
type InspectOptions struct {
	RemoteURL string
	LocalPath string
	Ref       string
	Runner    *Runner
}

// InspectRepo resolves metadata for a remote or local repository (no ingest).
func InspectRepo(ctx context.Context, opt InspectOptions) (*InspectReport, error) {
	r := opt.Runner
	if r == nil {
		r = &Runner{}
	}
	rep := &InspectReport{RequestedRef: opt.Ref}

	if opt.LocalPath != "" {
		return inspectLocal(ctx, r, opt.LocalPath, opt.Ref, rep)
	}
	if opt.RemoteURL == "" {
		return nil, fmt.Errorf("remoteUrl or localPath is required")
	}
	n, err := NormalizeRemoteURL(opt.RemoteURL)
	if err != nil {
		return nil, err
	}
	rep.RemoteURL = redactSecrets(n.CloneURL)
	rep.CloneURL = rep.RemoteURL
	rep.Provider = n.Provider
	rep.Owner = n.Owner
	rep.Repository = n.Repository
	if n.IsHTMLPage {
		rep.Warnings = append(rep.Warnings, "normalized GitHub HTML URL to clone URL")
	}

	ref := opt.Ref
	if ref == "" {
		// ls-remote HEAD
		out, err := r.Run(ctx, "", "ls-remote", "--symref", n.CloneURL, "HEAD")
		if err != nil {
			if looksAuthError(err) {
				rep.AuthRequired = true
			}
			rep.Warnings = append(rep.Warnings, err.Error())
			return rep, nil
		}
		rep.Accessible = true
		rep.DefaultBranch, rep.ResolvedCommitSHA = parseLSRemoteHEAD(out)
		rep.ResolvedRefType = "branch"
		if rep.DefaultBranch != "" {
			rep.RequestedRef = rep.DefaultBranch
		}
		return rep, nil
	}

	out, err := r.Run(ctx, "", "ls-remote", n.CloneURL, ref, "refs/heads/"+ref, "refs/tags/"+ref)
	if err != nil {
		if looksAuthError(err) {
			rep.AuthRequired = true
		}
		rep.Warnings = append(rep.Warnings, err.Error())
		return rep, nil
	}
	rep.Accessible = true
	sha, kind := pickRemoteRef(out, ref)
	rep.ResolvedCommitSHA = sha
	rep.ResolvedRefType = kind
	return rep, nil
}

func inspectLocal(ctx context.Context, r *Runner, path, ref string, rep *InspectReport) (*InspectReport, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local path is not a directory")
	}
	rep.LocalPath = abs
	if _, err := r.Run(ctx, abs, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", abs)
	}
	rep.Accessible = true
	rep.Provider = "local"
	if branch, err := r.Run(ctx, abs, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		rep.CurrentBranch = branch
		rep.DefaultBranch = branch
	}
	headRef := "HEAD"
	if ref != "" {
		headRef = ref
	}
	sha, err := r.Run(ctx, abs, "rev-parse", headRef)
	if err != nil {
		return nil, err
	}
	rep.ResolvedCommitSHA = sha
	rep.RequestedRef = headRef
	rep.ResolvedRefType = "commit"
	if st, err := r.Run(ctx, abs, "status", "--porcelain"); err == nil && st != "" {
		rep.WorkingTreeDirty = true
		for _, line := range strings.Split(st, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "??") {
				rep.UntrackedFiles++
			} else {
				rep.ModifiedFiles++
			}
		}
	}
	if sm, err := r.Run(ctx, abs, "submodule", "status"); err == nil && sm != "" {
		for _, line := range strings.Split(sm, "\n") {
			if strings.TrimSpace(line) != "" {
				rep.SubmoduleCount++
			}
		}
	}
	if remote, err := r.Run(ctx, abs, "remote", "get-url", "origin"); err == nil {
		rep.RemoteURL = redactSecrets(remote)
		if n, err := NormalizeRemoteURL(remote); err == nil {
			rep.Provider = n.Provider
			rep.Owner = n.Owner
			rep.Repository = n.Repository
		}
	}
	return rep, nil
}

func looksAuthError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "authentication") ||
		strings.Contains(s, "permission denied") ||
		strings.Contains(s, "could not read username") ||
		strings.Contains(s, "403") ||
		strings.Contains(s, "401")
}

func parseLSRemoteHEAD(out string) (branch, sha string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ref: refs/heads/") {
			rest := strings.TrimPrefix(line, "ref: refs/heads/")
			branch = strings.Fields(rest)[0]
		} else if fields := strings.Fields(line); len(fields) >= 2 && fields[1] == "HEAD" {
			sha = fields[0]
		}
	}
	return branch, sha
}

func pickRemoteRef(out, ref string) (sha, kind string) {
	wantTag := "refs/tags/" + ref
	wantHead := "refs/heads/" + ref
	var tagSHA, headSHA, exactSHA string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		switch {
		case name == wantTag || strings.HasPrefix(name, wantTag+"^{}"):
			if strings.HasSuffix(name, "^{}") {
				tagSHA = fields[0]
			} else if tagSHA == "" {
				tagSHA = fields[0]
			}
		case name == wantHead:
			headSHA = fields[0]
		case name == ref || len(ref) >= 7 && strings.HasPrefix(fields[0], ref):
			exactSHA = fields[0]
		}
	}
	switch {
	case tagSHA != "":
		return tagSHA, "tag"
	case headSHA != "":
		return headSHA, "branch"
	case exactSHA != "":
		return exactSHA, "commit"
	default:
		// first line fallback
		if fields := strings.Fields(out); len(fields) >= 1 {
			return fields[0], "commit"
		}
	}
	return "", ""
}
