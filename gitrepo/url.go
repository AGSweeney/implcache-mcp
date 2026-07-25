// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizedRemote is a validated clone URL plus identity fields.
type NormalizedRemote struct {
	CloneURL   string
	Provider   string
	Owner      string
	Repository string
	IsHTMLPage bool
}

// ErrLooksLikeGitRepo is returned when a web crawl URL should use repo tools.
var ErrLooksLikeGitRepo = fmt.Errorf("this URL appears to identify a Git repository; use add_repo_source or ingest_repo instead of ingest_site")

// LooksLikeGitRepoURL reports whether a documentation/crawl URL is a Git hosting page or clone URL.
func LooksLikeGitRepoURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.HasSuffix(lower, ".git") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := u.EscapedPath()
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		parts := splitPath(path)
		if len(parts) >= 2 {
			return true
		}
	case host == "gitlab.com" || strings.Contains(host, "gitlab"):
		if len(splitPath(path)) >= 2 {
			return true
		}
	case strings.HasPrefix(lower, "git@"):
		return true
	case u.Scheme == "ssh" || u.Scheme == "git":
		return true
	}
	return false
}

// NormalizeRemoteURL converts a user-supplied remote (including GitHub HTML URLs) into a clone URL.
func NormalizeRemoteURL(raw string) (*NormalizedRemote, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("remote url is required")
	}
	if strings.HasPrefix(raw, "git@") {
		// git@github.com:owner/repo.git
		rest := strings.TrimPrefix(raw, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if !ok {
			return nil, fmt.Errorf("invalid ssh git url")
		}
		path = strings.TrimSuffix(path, ".git")
		parts := splitPath(path)
		n := &NormalizedRemote{CloneURL: raw, Provider: providerForHost(host)}
		if len(parts) >= 2 {
			n.Owner, n.Repository = parts[0], parts[1]
		}
		return n, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse remote: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https", "http", "ssh", "git", "file":
	default:
		return nil, fmt.Errorf("unsupported git url scheme %q", u.Scheme)
	}
	if scheme == "file" {
		n := &NormalizedRemote{CloneURL: raw, Provider: "local"}
		return n, nil
	}
	if u.User != nil {
		// Allow but strip from stored identity; clone URL may keep for local file:// tests only.
	}
	host := strings.ToLower(u.Hostname())
	parts := splitPath(u.EscapedPath())
	n := &NormalizedRemote{Provider: providerForHost(host)}

	// GitHub HTML pages → clone URL
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		if len(parts) >= 2 {
			n.Owner, n.Repository = parts[0], strings.TrimSuffix(parts[1], ".git")
			if len(parts) > 2 {
				n.IsHTMLPage = true
			}
			n.CloneURL = "https://github.com/" + n.Owner + "/" + n.Repository + ".git"
			return n, nil
		}
	}

	path := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(strings.ToLower(path), ".git") && len(parts) >= 2 {
		// Accept https://host/owner/repo without .git
		n.Owner, n.Repository = parts[0], parts[1]
		u.Path = "/" + n.Owner + "/" + n.Repository + ".git"
		u.RawQuery = ""
		u.Fragment = ""
		n.CloneURL = u.String()
		return n, nil
	}
	if len(parts) >= 2 {
		n.Owner = parts[0]
		n.Repository = strings.TrimSuffix(parts[len(parts)-1], ".git")
	}
	u.RawQuery = ""
	u.Fragment = ""
	n.CloneURL = u.String()
	return n, nil
}

func providerForHost(host string) string {
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return "github"
	case host == "gitlab.com" || strings.Contains(host, "gitlab"):
		return "gitlab"
	case host == "" || host == "localhost" || host == "127.0.0.1":
		return "local"
	default:
		return "git"
	}
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(p, "/") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
