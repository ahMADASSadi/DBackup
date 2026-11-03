package security

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoundTrip(t *testing.T) {
	previous := chunkPlain
	chunkPlain = 32
	t.Cleanup(func() { chunkPlain = previous })

	payload := make([]byte, 200)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	copy(payload[:16], []byte("UNIQUE-MARKER!!"))
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.db")
	sealed := filepath.Join(dir, "backup.dbk")
	opened := filepath.Join(dir, "opened.db")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	password := "correct-horse-battery"
	if err := SealFile(sealed, src, password); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("UNIQUE-MARKER!!")) {
		t.Fatal("plaintext leaked into the archive")
	}
	if err := OpenFile(opened, sealed, password); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(opened)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round trip changed %d bytes into %d", len(payload), len(got))
	}
}

func TestArchiveWrongPassword(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "plain.db")
	sealed := filepath.Join(dir, "backup.dbk")
	opened := filepath.Join(dir, "opened.db")
	if err := os.WriteFile(src, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SealFile(sealed, src, "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	if err := OpenFile(opened, sealed, "another-horse-battery"); err == nil {
		t.Fatal("expected the wrong password to fail")
	}
	if _, err := os.Stat(opened); !os.IsNotExist(err) {
		t.Fatal("expected a failed decrypt to leave no output file")
	}
}

func TestVaultHoldsPasswordOnlyInMemory(t *testing.T) {
	vault := NewVault()
	if vault.Unlocked() {
		t.Fatal("new vault is unlocked")
	}
	if _, ok := vault.Password(); ok {
		t.Fatal("empty vault returned a password")
	}
	vault.Set("correct-horse-battery")
	if !vault.Unlocked() {
		t.Fatal("expected vault to unlock")
	}
	got, ok := vault.Password()
	if !ok || got != "correct-horse-battery" {
		t.Fatalf("password %q ok=%v", got, ok)
	}
	vault.Clear()
	if vault.Unlocked() {
		t.Fatal("expected vault to lock")
	}
	vault.Set("correct-horse-battery")
	vault.Set("")
	if vault.Unlocked() {
		t.Fatal("empty password should lock the vault")
	}
}
