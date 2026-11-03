// Package storage publishes backup files and uploads them to object storage.
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dbackup/internal/security"
)

// Local is the application-managed backup directory.
type Local struct {
	root string
}

// NewLocal creates the backup directory and its temporary area.
func NewLocal(root string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("backup directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, ".tmp"), 0o700); err != nil {
		return nil, fmt.Errorf("create backup temporary directory: %w", err)
	}
	return &Local{root: abs}, nil
}

// Root returns the absolute backup directory.
func (l *Local) Root() string { return l.root }

// CreateTemp reserves an empty file for an in-progress dump.
func (l *Local) CreateTemp(backupID, ext string) (string, error) {
	if !security.ValidID(backupID) || !validExt(ext) {
		return "", fmt.Errorf("invalid temporary backup name")
	}
	path := filepath.Join(l.root, ".tmp", backupID+ext+".partial")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create temporary backup: %w", err)
	}
	return path, file.Close()
}

// Publish hashes a completed temporary file and atomically moves it into place.
func (l *Local) Publish(tempPath, rel string) (int64, string, error) {
	if err := l.ensureTemp(tempPath); err != nil {
		return 0, "", err
	}
	dest, err := l.resolve(rel)
	if err != nil {
		return 0, "", err
	}
	size, sum, err := hashFile(tempPath)
	if err != nil {
		return 0, "", err
	}
	if size == 0 {
		return 0, "", fmt.Errorf("backup file is empty")
	}
	if err := syncFile(tempPath); err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return 0, "", fmt.Errorf("create backup folder: %w", err)
	}
	if err := os.Rename(tempPath, dest); err != nil {
		return 0, "", fmt.Errorf("publish backup: %w", err)
	}
	if err := os.Chmod(dest, 0o600); err != nil {
		return 0, "", err
	}
	return size, sum, nil
}

// Path resolves a published relative key to an absolute path inside the backup directory.
func (l *Local) Path(rel string) (string, error) { return l.resolve(rel) }

// Open opens a published backup for download.
func (l *Local) Open(rel string) (*os.File, os.FileInfo, error) {
	path, err := l.resolve(rel)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("backup is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return file, info, nil
}

// Remove deletes a published backup. Missing files are ignored.
func (l *Local) Remove(rel string) error {
	if rel == "" {
		return nil
	}
	path, err := l.resolve(rel)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// RemoveTemp deletes one temporary file inside the temporary directory.
func (l *Local) RemoveTemp(path string) error {
	if path == "" {
		return nil
	}
	if err := l.ensureTemp(path); err != nil {
		return err
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CleanupTemp removes leftover temporary files after a restart.
func (l *Local) CleanupTemp() error {
	dir := filepath.Join(l.root, ".tmp")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Usage sums regular files under the backup directory, excluding temporary files.
func (l *Local) Usage() (int64, error) {
	var total int64
	err := filepath.WalkDir(l.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(l.root, path)
		if err != nil {
			return err
		}
		if rel == ".tmp" || strings.HasPrefix(rel, ".tmp"+string(filepath.Separator)) {
			if d.IsDir() && rel == ".tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// CredentialDir creates a private directory for short-lived client credential files.
func (l *Local) CredentialDir() (string, error) {
	return os.MkdirTemp(filepath.Join(l.root, ".tmp"), "cred-")
}

func (l *Local) resolve(rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "\x00") {
		return "", fmt.Errorf("backup path is invalid")
	}
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup path is invalid")
	}
	full := filepath.Join(l.root, clean)
	relative, err := filepath.Rel(l.root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup path is invalid")
	}
	if relative == ".tmp" || strings.HasPrefix(relative, ".tmp"+string(filepath.Separator)) {
		return "", fmt.Errorf("backup path is invalid")
	}
	return full, nil
}

func (l *Local) ensureTemp(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("temporary backup path is invalid")
	}
	tempRoot := filepath.Join(l.root, ".tmp")
	rel, err := filepath.Rel(tempRoot, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("temporary backup path is invalid")
	}
	return nil
}

func hashFile(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func syncFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func validExt(ext string) bool {
	switch ext {
	case ".dump", ".sql", ".db", ".dbk":
		return true
	default:
		return false
	}
}
