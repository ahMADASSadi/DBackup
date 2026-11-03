package security

import "sync"

// Vault keeps the administrator password in process memory so scheduled
// backups can encrypt files. It is never written to disk.
type Vault struct {
	mu     sync.Mutex
	secret []byte
}

// NewVault returns an empty vault.
func NewVault() *Vault { return &Vault{} }

// Set replaces the in-memory password.
func (v *Vault) Set(password string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	wipe(v.secret)
	v.secret = nil
	if password == "" {
		return
	}
	v.secret = append([]byte(nil), password...)
}

// Clear wipes the in-memory password.
func (v *Vault) Clear() {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	wipe(v.secret)
	v.secret = nil
}

// Unlocked reports whether a password is currently held.
func (v *Vault) Unlocked() bool {
	if v == nil {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.secret) > 0
}

// Password returns a copy of the held password.
func (v *Vault) Password() (string, bool) {
	if v == nil {
		return "", false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.secret) == 0 {
		return "", false
	}
	return string(v.secret), true
}
