package backup

import (
	"context"
	"fmt"
	"os"

	"dbackup/internal/model"
	"dbackup/internal/security"
)

// Engine dumps databases and tests connections.
type Engine struct {
	Runner    Runner
	Roots     []string
	Allowlist []string
	Resolver  security.Resolver
	MetaPath  string
}

// ToolStatus reports whether native clients are installed.
type ToolStatus struct {
	PgDump    string `json:"pg_dump"`
	MySQLDump string `json:"mysqldump"`
	PgError   string `json:"pg_dump_error"`
	MyError   string `json:"mysqldump_error"`
}

// CheckTools looks up the native clients without failing the process.
func (e *Engine) CheckTools(ctx context.Context) ToolStatus {
	runner := e.runner()
	status := ToolStatus{}
	if _, err := runner.LookPath("pg_dump"); err != nil {
		status.PgError = toolMissing("pg_dump").Error()
	} else if out, err := runner.Run(ctx, "pg_dump", []string{"--version"}, nil); err != nil {
		status.PgError = "pg_dump --version failed"
	} else {
		status.PgDump = firstLine(out.Stdout)
	}
	if _, err := runner.LookPath("mysqldump"); err != nil {
		status.MyError = toolMissing("mysqldump").Error()
	} else if out, err := runner.Run(ctx, "mysqldump", []string{"--version"}, nil); err != nil {
		status.MyError = "mysqldump --version failed"
	} else {
		status.MySQLDump = firstLine(out.Stdout + out.Stderr)
	}
	return status
}

// Ping verifies that a source accepts a connection.
func (e *Engine) Ping(ctx context.Context, src model.Source, password string) error {
	src, err := e.prepare(ctx, src)
	if err != nil {
		return err
	}
	switch src.Type {
	case model.TypeSQLite:
		return pingSQLite(ctx, src.Config.Path)
	case model.TypePostgres, model.TypeMySQL:
		dir, err := os.MkdirTemp("", "dbackup-ping-")
		if err != nil {
			return fmt.Errorf("prepare connection test")
		}
		defer os.RemoveAll(dir)
		if src.Type == model.TypePostgres {
			return pingPostgres(ctx, src.Config, password, dir)
		}
		return pingMySQL(ctx, src.Config, password)
	default:
		return fmt.Errorf("unsupported database type")
	}
}

// Dump writes a consistent backup to dest.
func (e *Engine) Dump(ctx context.Context, src model.Source, password, dest, credParent string) (string, error) {
	src, err := e.prepare(ctx, src)
	if err != nil {
		return "", err
	}
	switch src.Type {
	case model.TypeSQLite:
		return dumpSQLite(ctx, src.Config.Path, dest)
	case model.TypePostgres, model.TypeMySQL:
		dir, err := os.MkdirTemp(credParent, "cred-")
		if err != nil {
			return "", fmt.Errorf("prepare backup credentials")
		}
		defer os.RemoveAll(dir)
		if src.Type == model.TypePostgres {
			return dumpPostgres(ctx, e.runner(), dir, src.Config, password, dest)
		}
		return dumpMySQL(ctx, e.runner(), dir, src.Config, password, dest)
	default:
		return "", fmt.Errorf("unsupported database type")
	}
}

func (e *Engine) prepare(ctx context.Context, src model.Source) (model.Source, error) {
	switch src.Type {
	case model.TypeSQLite:
		resolved, err := security.ValidateSQLitePath(src.Config.Path, e.Roots)
		if err != nil {
			return src, err
		}
		if e.MetaPath != "" && resolved == e.MetaPath {
			return src, fmt.Errorf("the metadata database cannot be configured as a source")
		}
		src.Config.Path = resolved
		return src, nil
	case model.TypePostgres, model.TypeMySQL:
		if err := security.ResolveHost(ctx, e.Resolver, src.Config.Host, e.Allowlist); err != nil {
			return src, err
		}
		return src, nil
	default:
		return src, fmt.Errorf("unsupported database type")
	}
}

func (e *Engine) runner() Runner {
	if e.Runner != nil {
		return e.Runner
	}
	return OSRunner{}
}
