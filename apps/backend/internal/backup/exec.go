package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result is captured process output, truncated by the runner.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes a backup utility without a shell.
type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args []string, extraEnv []string) (Result, error)
}

// OSRunner uses os/exec with argument arrays.
type OSRunner struct{}

func (OSRunner) LookPath(file string) (string, error) { return exec.LookPath(file) }

func (OSRunner) Run(ctx context.Context, name string, args []string, extraEnv []string) (Result, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = append(stripEnv(os.Environ(), "PGPASSWORD", "MYSQL_PWD"), extraEnv...)
	command.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout = &limitWriter{w: &stdout, n: 8192}
	command.Stderr = &limitWriter{w: &stderr, n: 8192}
	err := command.Run()
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func stripEnv(env []string, keys ...string) []string {
	blocked := map[string]struct{}{}
	for _, key := range keys {
		blocked[key] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if _, skip := blocked[name]; skip {
			continue
		}
		out = append(out, item)
	}
	return out
}

type limitWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		p = p[:l.n]
	}
	n, err := l.w.Write(p)
	l.n -= n
	if err != nil {
		return n, err
	}
	return len(p), nil
}

var _ io.Writer = (*limitWriter)(nil)

func toolMissing(name string) error {
	switch name {
	case "pg_dump":
		return fmt.Errorf("pg_dump is not installed or not on PATH; install a PostgreSQL client at least as new as the server")
	case "mysqldump":
		return fmt.Errorf("mysqldump is not installed or not on PATH; install a MySQL or MariaDB client")
	default:
		return fmt.Errorf("%s is not installed or not on PATH", name)
	}
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	line, _, _ := strings.Cut(value, "\n")
	line = strings.TrimSpace(line)
	if len(line) > 160 {
		line = line[:160]
	}
	return line
}
