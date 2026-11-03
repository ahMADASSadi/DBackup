package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"dbackup/internal/config"
	"dbackup/internal/repository"
	"dbackup/internal/security"
)

const loginWindow = 15 * time.Minute
const loginLimit = 8

// Auth manages the single administrator session.
type Auth struct {
	Store *repository.Store
	Dummy string
	TTL   time.Duration
	Vault *security.Vault
}

// NewAuth prepares a dummy hash so missing accounts do not return early.
func NewAuth(store *repository.Store, ttl time.Duration, vault *security.Vault) (*Auth, error) {
	dummy, err := security.DummyHash()
	if err != nil {
		return nil, err
	}
	if ttl < time.Minute {
		ttl = 12 * time.Hour
	}
	if vault == nil {
		vault = security.NewVault()
	}
	return &Auth{Store: store, Dummy: dummy, TTL: ttl, Vault: vault}, nil
}

// Bootstrap creates the administrator or applies a one-shot password reset.
func (a *Auth) Bootstrap(ctx context.Context, cfg config.Config) error {
	admin, err := a.Store.GetAdmin(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		if err := security.ValidateAdminUsername(cfg.AdminUsername); err != nil {
			return errors.New("set ADMIN_USERNAME to a valid account name before starting")
		}
		if err := security.ValidatePassword(cfg.AdminPassword); err != nil {
			return errors.New("set ADMIN_PASSWORD to at least 12 characters before starting")
		}
		hash, err := security.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		return a.Store.CreateAdmin(ctx, cfg.AdminUsername, hash)
	}
	if err != nil {
		return err
	}
	if !cfg.AdminPasswordReset {
		return nil
	}
	if err := security.ValidatePassword(cfg.AdminPassword); err != nil {
		return errors.New("ADMIN_PASSWORD_RESET requires ADMIN_PASSWORD of at least 12 characters")
	}
	hash, err := security.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	if err := a.Store.UpdateAdminPassword(ctx, hash); err != nil {
		return err
	}
	if err := a.Store.DeleteAllSessions(ctx); err != nil {
		return err
	}
	_ = a.Store.InsertAudit(ctx, admin.Username, "admin.password_reset", "admin", admin.ID, "{}")
	return nil
}

// LoginResult is returned to the HTTP layer. The token is placed in a cookie and is not logged.
type LoginResult struct {
	Username  string
	Token     string
	CSRF      string
	ExpiresAt time.Time
}

// Login checks the rate limit and password, then replaces any existing session.
func (a *Auth) Login(ctx context.Context, username, password, ip string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	_ = a.Store.PruneLoginAttempts(ctx, time.Now().Add(-24*time.Hour))
	ipCount, userCount, err := a.Store.CountFailedLogins(ctx, ip, username, time.Now().Add(-loginWindow))
	if err != nil {
		return LoginResult{}, err
	}
	limited := ipCount >= loginLimit || userCount >= loginLimit
	admin, err := a.Store.GetAdmin(ctx)
	hash := a.Dummy
	known := err == nil
	if known {
		hash = admin.PasswordHash
	} else if !errors.Is(err, sql.ErrNoRows) {
		return LoginResult{}, err
	}
	valid := security.VerifyPassword(password, hash) && known && admin.Username == username
	if limited {
		_ = a.Store.InsertLoginAttempt(ctx, ip, username, false)
		return LoginResult{}, E(429, "rate_limited", "too many login attempts; try again later")
	}
	if !valid {
		_ = a.Store.InsertLoginAttempt(ctx, ip, username, false)
		return LoginResult{}, E(401, "unauthorized", "invalid username or password")
	}
	if err := a.Store.InsertLoginAttempt(ctx, ip, username, true); err != nil {
		return LoginResult{}, err
	}
	if err := a.Store.DeleteAllSessions(ctx); err != nil {
		return LoginResult{}, err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return LoginResult{}, err
	}
	csrf, _, err := security.NewToken()
	if err != nil {
		return LoginResult{}, err
	}
	expires := time.Now().Add(a.TTL)
	if _, err := a.Store.CreateSession(ctx, tokenHash, csrf, expires); err != nil {
		return LoginResult{}, err
	}
	a.Vault.Set(password)
	_ = a.Store.InsertAudit(ctx, username, "auth.login", "admin", admin.ID, "{}")
	return LoginResult{Username: username, Token: token, CSRF: csrf, ExpiresAt: expires}, nil
}

// Session loads an unexpired session for a raw cookie token.
func (a *Auth) Session(ctx context.Context, token string) (repository.Session, string, error) {
	if token == "" {
		return repository.Session{}, "", E(401, "unauthorized", "authentication required")
	}
	session, err := a.Store.GetSession(ctx, security.HashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return repository.Session{}, "", E(401, "unauthorized", "authentication required")
	}
	if err != nil {
		return repository.Session{}, "", err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = a.Store.DeleteSession(ctx, session.ID)
		a.dropIfIdle(ctx)
		return repository.Session{}, "", E(401, "unauthorized", "authentication required")
	}
	admin, err := a.Store.GetAdmin(ctx)
	if err != nil {
		return repository.Session{}, "", err
	}
	return session, admin.Username, nil
}

// Logout deletes the current session.
func (a *Auth) Logout(ctx context.Context, sessionID, username string) error {
	if err := a.Store.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	a.dropIfIdle(ctx)
	_ = a.Store.InsertAudit(ctx, username, "auth.logout", "admin", sessionID, "{}")
	return nil
}

// Unlock stores the administrator password in memory after checking it.
func (a *Auth) Unlock(ctx context.Context, password string) error {
	admin, err := a.Store.GetAdmin(ctx)
	if err != nil {
		return err
	}
	if !security.VerifyPassword(password, admin.PasswordHash) {
		return E(401, "unauthorized", "invalid password")
	}
	a.Vault.Set(password)
	return nil
}

func (a *Auth) dropIfIdle(ctx context.Context) {
	n, err := a.Store.CountLiveSessions(ctx, time.Now())
	if err == nil && n == 0 {
		a.Vault.Clear()
	}
}

// ChangePassword replaces the administrator password and invalidates sessions.
func (a *Auth) ChangePassword(ctx context.Context, current, next string) error {
	admin, err := a.Store.GetAdmin(ctx)
	if err != nil {
		return err
	}
	if !security.VerifyPassword(current, admin.PasswordHash) {
		return E(401, "unauthorized", "current password is incorrect")
	}
	if err := security.ValidatePassword(next); err != nil {
		return E(400, "validation_error", err.Error())
	}
	if current == next {
		return E(400, "validation_error", "new password must be different")
	}
	hash, err := security.HashPassword(next)
	if err != nil {
		return err
	}
	if err := a.Store.UpdateAdminPassword(ctx, hash); err != nil {
		return err
	}
	if err := a.Store.DeleteAllSessions(ctx); err != nil {
		return err
	}
	a.Vault.Clear()
	_ = a.Store.InsertAudit(ctx, admin.Username, "admin.password_change", "admin", admin.ID, "{}")
	return nil
}
