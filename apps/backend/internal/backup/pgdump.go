package backup

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

// ponytail: scan ceiling for versioned client paths; raise when PostgreSQL ships past it.
const pgDumpMajorScanMax = 22

func selectPgDump(ctx context.Context, runner Runner, serverMajor int) (bin, version string, err error) {
	if serverMajor < 1 {
		return "", "", fmt.Errorf("could not determine postgres server version")
	}
	for major := pgDumpMajorScanMax; major >= serverMajor; major-- {
		path := fmt.Sprintf("/usr/lib/postgresql/%d/bin/pg_dump", major)
		if _, lookErr := runner.LookPath(path); lookErr != nil {
			continue
		}
		out, runErr := runner.Run(ctx, path, []string{"--version"}, nil)
		if runErr != nil {
			continue
		}
		return path, firstLine(out.Stdout), nil
	}
	if _, lookErr := runner.LookPath("pg_dump"); lookErr == nil {
		out, runErr := runner.Run(ctx, "pg_dump", []string{"--version"}, nil)
		if runErr == nil {
			version = firstLine(out.Stdout)
			if maj, ok := parsePgMajor(version); ok && maj >= serverMajor {
				return "pg_dump", version, nil
			}
			return "", "", fmt.Errorf("pg_dump (%s) is older than server major %d; rebuild with PG_MAJOR=%d or newer", version, serverMajor, serverMajor)
		}
	}
	return "", "", fmt.Errorf("no pg_dump >= %d found; rebuild with PG_MAJOR=%d or newer", serverMajor, serverMajor)
}

func postgresServerMajor(ctx context.Context, runner Runner, credDir string, cfg model.ConnConfig, password string) (int, error) {
	if _, err := runner.LookPath("psql"); err != nil {
		return 0, fmt.Errorf("psql is required to detect the postgres server version")
	}
	passFile, err := writePgpass(credDir, cfg, password)
	if err != nil {
		return 0, err
	}
	args := []string{
		"--no-password",
		"--tuples-only",
		"--no-align",
		"--command", "SHOW server_version_num",
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--username", cfg.Username,
		"--dbname", cfg.Database,
	}
	env := []string{
		"PGPASSFILE=" + passFile,
		"PGSSLMODE=" + pgSSLMode(cfg.TLSMode),
	}
	if timeout := cfg.Params["connect_timeout"]; timeout != "" {
		env = append(env, "PGCONNECT_TIMEOUT="+timeout)
	}
	if cfg.TLSMode == "verify-ca" {
		caPath, err := writeCA(credDir, cfg.TLSCAPem)
		if err != nil {
			return 0, err
		}
		env = append(env, "PGSSLROOTCERT="+caPath)
	}
	result, err := runner.Run(ctx, "psql", args, env)
	if err != nil {
		detail := security.Truncate(security.Redact(strings.TrimSpace(result.Stderr+" "+result.Stdout), password), 300)
		if detail == "" {
			detail = "could not read postgres server version"
		}
		return 0, fmt.Errorf("%s", detail)
	}
	major, ok := parsePgMajor(firstLine(result.Stdout))
	if !ok {
		return 0, fmt.Errorf("could not parse postgres server version %q", firstLine(result.Stdout))
	}
	return major, nil
}

func parsePgMajor(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	// server_version_num like 180006
	if n, err := strconv.Atoi(text); err == nil {
		if n >= 10000 {
			return n / 10000, true
		}
		if n >= 8 && n < 100 {
			return n, true
		}
	}
	// pg_dump (PostgreSQL) 18.6 (Debian ...)
	for _, part := range strings.Fields(text) {
		part = strings.Trim(part, "(),")
		if i := strings.IndexByte(part, '.'); i > 0 {
			part = part[:i]
		}
		if n, err := strconv.Atoi(part); err == nil && n >= 8 && n < 100 {
			return n, true
		}
	}
	return 0, false
}
