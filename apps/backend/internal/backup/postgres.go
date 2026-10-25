package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

func dumpPostgres(ctx context.Context, runner Runner, credDir string, cfg model.ConnConfig, password, dest string) (string, error) {
	serverMajor, err := postgresServerMajor(ctx, runner, credDir, cfg, password)
	if err != nil {
		return "", err
	}
	bin, version, err := selectPgDump(ctx, runner, serverMajor)
	if err != nil {
		return "", err
	}
	passFile, err := writePgpass(credDir, cfg, password)
	if err != nil {
		return "", err
	}
	args := []string{
		"--format=custom",
		"--no-password",
		"--file", dest,
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
			return "", err
		}
		env = append(env, "PGSSLROOTCERT="+caPath)
	}
	result, err := runner.Run(ctx, bin, args, env)
	if err != nil {
		detail := security.Truncate(security.Redact(strings.TrimSpace(result.Stderr+" "+result.Stdout), password), 400)
		if detail == "" {
			detail = "pg_dump failed"
		}
		return version, fmt.Errorf("%s", detail)
	}
	return version, nil
}

func pgSSLMode(mode string) string {
	switch mode {
	case "disable":
		return "disable"
	case "verify-ca":
		return "verify-ca"
	default:
		return "require"
	}
}

func writePgpass(dir string, cfg model.ConnConfig, password string) (string, error) {
	line := strings.Join([]string{
		escapePgpass(cfg.Host),
		strconv.Itoa(cfg.Port),
		escapePgpass(cfg.Database),
		escapePgpass(cfg.Username),
		escapePgpass(password),
	}, ":") + "\n"
	path := filepath.Join(dir, "pgpass")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		return "", fmt.Errorf("write postgres credential file: %w", err)
	}
	return path, nil
}

func escapePgpass(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `:`, `\:`)
}

func writeCA(dir, pem string) (string, error) {
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, []byte(pem), 0o600); err != nil {
		return "", fmt.Errorf("write tls ca file: %w", err)
	}
	return path, nil
}
