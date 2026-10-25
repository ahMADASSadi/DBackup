package backup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"dbackup/internal/model"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func pingPostgres(ctx context.Context, cfg model.ConnConfig, password, credDir string) error {
	query := url.Values{}
	query.Set("sslmode", pgSSLMode(cfg.TLSMode))
	if timeout := cfg.Params["connect_timeout"]; timeout != "" {
		query.Set("connect_timeout", timeout)
	}
	if name := cfg.Params["application_name"]; name != "" {
		query.Set("application_name", name)
	}
	if cfg.TLSMode == "verify-ca" {
		caPath, err := writeCA(credDir, cfg.TLSCAPem)
		if err != nil {
			return err
		}
		query.Set("sslrootcert", caPath)
	}
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.Username, password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:     "/" + cfg.Database,
		RawQuery: query.Encode(),
	}).String()
	return pingSQL(ctx, "pgx", dsn)
}

func pingMySQL(ctx context.Context, cfg model.ConnConfig, password string) error {
	tlsName := "false"
	switch cfg.TLSMode {
	case "disable":
		tlsName = "false"
	case "verify-ca":
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.TLSCAPem)) {
			return fmt.Errorf("tls ca bundle is invalid")
		}
		tlsName = "dbackup-verify-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if err := mysql.RegisterTLSConfig(tlsName, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}); err != nil {
			return fmt.Errorf("tls configuration failed")
		}
		defer mysql.DeregisterTLSConfig(tlsName)
	default:
		tlsName = "dbackup-require-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if err := mysql.RegisterTLSConfig(tlsName, &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}); err != nil {
			return fmt.Errorf("tls configuration failed")
		}
		defer mysql.DeregisterTLSConfig(tlsName)
	}
	conf := mysql.NewConfig()
	conf.User = cfg.Username
	conf.Passwd = password
	conf.Net = "tcp"
	conf.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conf.DBName = cfg.Database
	conf.Timeout = 10 * time.Second
	conf.ReadTimeout = 10 * time.Second
	conf.WriteTimeout = 10 * time.Second
	conf.TLSConfig = tlsName
	conf.Params = map[string]string{}
	if charset := cfg.Params["charset"]; charset != "" {
		conf.Params["charset"] = charset
	}
	return pingSQL(ctx, "mysql", conf.FormatDSN())
}

func pingSQLite(ctx context.Context, path string) error {
	return pingSQL(ctx, "sqlite", sqliteReadDSN(path))
}

func pingSQL(ctx context.Context, driver, dsn string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("connection failed")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("connection failed")
	}
	if _, err := db.ExecContext(pingCtx, `SELECT 1`); err != nil {
		return fmt.Errorf("connection failed")
	}
	return nil
}
