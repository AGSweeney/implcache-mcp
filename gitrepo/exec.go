// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultGitTimeout = 5 * time.Minute
	maxGitOutput      = 4 << 20
)

// Runner executes git with safety defaults.
type Runner struct {
	GitPath string
	Timeout time.Duration
	Env     []string
}

func (r *Runner) gitBin() string {
	if r != nil && r.GitPath != "" {
		return r.GitPath
	}
	return "git"
}

func (r *Runner) timeout() time.Duration {
	if r != nil && r.Timeout > 0 {
		return r.Timeout
	}
	return defaultGitTimeout
}

// Run executes git args in dir and returns stdout.
func (r *Runner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	full := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "protocol.file.allow=always"}, args...)
	cmd := exec.CommandContext(ctx, r.gitBin(), full...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.hooksPath",
		"GIT_CONFIG_VALUE_0=/dev/null",
	)
	if r != nil && len(r.Env) > 0 {
		cmd.Env = append(cmd.Env, r.Env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{buf: &stdout, max: maxGitOutput}
	cmd.Stderr = &limitedWriter{buf: &stderr, max: maxGitOutput}
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("git %s: %s", redactArgs(args), redactSecrets(msg))
	}
	return out, nil
}

type limitedWriter struct {
	buf *bytes.Buffer
	max int
	n   int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.n >= w.max {
		return len(p), nil
	}
	remain := w.max - w.n
	if len(p) > remain {
		p = p[:remain]
	}
	n, err := w.buf.Write(p)
	w.n += n
	return len(p), err
}

func redactArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = redactSecrets(a)
	}
	return strings.Join(out, " ")
}

func redactSecrets(s string) string {
	// Strip userinfo from URLs: https://user:token@host → https://***@host
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		if at := strings.Index(rest, "@"); at > 0 {
			if strings.Contains(rest[:at], ":") || strings.Contains(rest[:at], "%") {
				return s[:i+3] + "***@" + rest[at+1:]
			}
		}
	}
	return s
}
