package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

func dumpMySQL(ctx context.Context, runner Runner, credDir string, cfg model.ConnConfig, password, dest string) (string, error) {
	if _, err := runner.LookPath("mysqldump"); err != nil {
		return "", toolMissing("mysqldump")
	}
	versionOut, err := runner.Run(ctx, "mysqldump", []string{"--version"}, nil)
	if err != nil {
		return "", fmt.Errorf("mysqldump --version failed")
	}
	version := firstLine(versionOut.Stdout + versionOut.Stderr)
	maria := strings.Contains(strings.ToLower(version), "mariadb")
	if err := writeMySQLDefaults(credDir, cfg, password, maria); err != nil {
		return "", err
	}
	args := []string{
		"--defaults-extra-file=" + filepath.Join(credDir, "client.cnf"),
		"--single-transaction",
		"--routines",
		"--triggers",
		"--hex-blob",
		"--result-file=" + dest,
	}
	if !maria {
		args = append(args, "--column-statistics=0", "--set-gtid-purged=OFF")
	}
	args = append(args, cfg.Database)
	result, err := runner.Run(ctx, "mysqldump", args, nil)
	if err != nil {
		detail := security.Truncate(security.Redact(strings.TrimSpace(result.Stderr+" "+result.Stdout), password), 400)
		if detail == "" {
			detail = "mysqldump failed"
		}
		return version, fmt.Errorf("%s", detail)
	}
	return version, nil
}

func writeMySQLDefaults(dir string, cfg model.ConnConfig, password string, maria bool) error {
	var b strings.Builder
	b.WriteString("[client]\n")
	fmt.Fprintf(&b, "host=%s\n", quoteOption(cfg.Host))
	fmt.Fprintf(&b, "port=%d\n", cfg.Port)
	fmt.Fprintf(&b, "user=%s\n", quoteOption(cfg.Username))
	fmt.Fprintf(&b, "password=%s\n", quoteOption(password))
	switch cfg.TLSMode {
	case "disable":
		if maria {
			b.WriteString("skip-ssl\n")
		} else {
			b.WriteString("ssl-mode=DISABLED\n")
		}
	case "verify-ca":
		caPath := filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(caPath, []byte(cfg.TLSCAPem), 0o600); err != nil {
			return fmt.Errorf("write tls ca file: %w", err)
		}
		if maria {
			fmt.Fprintf(&b, "ssl=1\nssl-ca=%s\nssl-verify-server-cert\n", quoteOption(caPath))
		} else {
			fmt.Fprintf(&b, "ssl-mode=VERIFY_CA\nssl-ca=%s\n", quoteOption(caPath))
		}
	default:
		if maria {
			b.WriteString("ssl=1\n")
		} else {
			b.WriteString("ssl-mode=REQUIRED\n")
		}
	}
	path := filepath.Join(dir, "client.cnf")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func quoteOption(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
