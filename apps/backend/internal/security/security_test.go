package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct-horse-battery", hash) {
		t.Fatal("expected password to match")
	}
	if VerifyPassword("wrong-horse-battery", hash) {
		t.Fatal("expected password to fail")
	}
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("expected short password to be rejected")
	}
}

func TestCipherBindsAAD(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipher, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := cipher.Encrypt("source:one", "s3cret-value")
	if err != nil {
		t.Fatal(err)
	}
	got, err := cipher.Decrypt("source:one", sealed)
	if err != nil || got != "s3cret-value" {
		t.Fatalf("decrypt: %v %q", err, got)
	}
	if _, err := cipher.Decrypt("source:two", sealed); err == nil {
		t.Fatal("expected aad mismatch to fail")
	}
}

func TestRejectsShellHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1;touch /tmp/x", "$(id)", "host name", "http://db", "metadata.google.internal", "169.254.169.254"} {
		if err := ValidateHostSyntax(host); err == nil {
			t.Fatalf("accepted %q", host)
		}
	}
	if err := ResolveHost(context.Background(), nil, "127.0.0.1", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSQLitePathEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	allowed := filepath.Join(root, "db.sqlite")
	if err := os.WriteFile(allowed, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ValidateSQLitePath(allowed, []string{root})
	if err != nil || got != allowed {
		t.Fatalf("allowed path: %v %s", err, got)
	}
	secret := filepath.Join(outside, "secret.db")
	if err := os.WriteFile(secret, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSQLitePath(secret, []string{root}); err == nil {
		t.Fatal("accepted path outside the root")
	}
	link := filepath.Join(root, "link.db")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateSQLitePath(link, []string{root}); err == nil {
		t.Fatal("accepted symlink escape")
	}
}

func TestBackupDirOverlap(t *testing.T) {
	if err := ValidateDataPaths("/data/backups", "/data/meta/dbackup.db"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDataPaths("/data", "/data/meta/dbackup.db"); err == nil {
		t.Fatal("expected overlap to be rejected")
	}
	if err := ValidateDataPaths("/etc/dbackup", "/data/meta/dbackup.db"); err == nil {
		t.Fatal("expected unsafe directory to be rejected")
	}
}

func TestRedact(t *testing.T) {
	got := Redact("password=hunter2 token=abc", "hunter2")
	if got == "password=hunter2 token=abc" || contains(got, "hunter2") {
		t.Fatal(got)
	}
}

func contains(s, part string) bool {
	return len(part) > 0 && len(s) >= len(part) && (s == part || len(s) > 0 && (stringIndex(s, part) >= 0))
}

func stringIndex(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
