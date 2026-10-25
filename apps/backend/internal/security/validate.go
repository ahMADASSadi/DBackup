package security

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	hostName = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*)$`)
	ident    = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{0,62}$`)
	userName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,62}$`)
	appName  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	duration = regexp.MustCompile(`^[1-9][0-9]{0,3}(ms|s)$`)
	charset  = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
	bucketRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{1,61}[A-Za-z0-9]$`)
)

var blockedNames = map[string]struct{}{
	"metadata.google.internal": {},
	"metadata.google.com":      {},
	"instance-data":            {},
}

// Resolver looks up a database or object-storage host.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// SystemResolver uses the Go resolver.
type SystemResolver struct{}

func (SystemResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, network, host)
}

// ValidateHostSyntax rejects values that could change process arguments or URLs.
func ValidateHostSyntax(host string) error {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return fmt.Errorf("host is required")
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, " \t\r\n\\/'\"`$&;|<>(){}[]!*?") {
		return fmt.Errorf("host contains unsupported characters")
	}
	if _, blocked := blockedNames[strings.ToLower(host)]; blocked {
		return fmt.Errorf("host is not allowed")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if err := rejectIP(ip); err != nil {
			return err
		}
		return nil
	}
	if !hostName.MatchString(host) {
		return fmt.Errorf("host must be a hostname or IP address")
	}
	return nil
}

// ResolveHost checks DNS results and an optional allowlist.
// Allowlist entries are hostnames or CIDRs. An empty allowlist still blocks
// link-local and metadata addresses.
func ResolveHost(ctx context.Context, resolver Resolver, host string, allowlist []string) error {
	if err := ValidateHostSyntax(host); err != nil {
		return err
	}
	if !allowlisted(host, allowlist) {
		return fmt.Errorf("host is not in the allowlist")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return rejectIP(ip)
	}
	if resolver == nil {
		resolver = SystemResolver{}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := resolver.LookupNetIP(lookupCtx, "ip", host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("host could not be resolved")
	}
	for _, addr := range addrs {
		if err := rejectIP(addr); err != nil {
			return err
		}
	}
	return nil
}

func allowlisted(host string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ip, ipErr := netip.ParseAddr(host)
	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, host) {
			return true
		}
		prefix, err := netip.ParsePrefix(item)
		if err == nil && ipErr == nil && prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func rejectIP(ip netip.Addr) error {
	if !ip.IsValid() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("host is not allowed")
	}
	// Cloud metadata addresses that are not covered above.
	if ip.String() == "169.254.169.254" || ip.String() == "fd00:ec2::254" {
		return fmt.Errorf("host is not allowed")
	}
	return nil
}

// ValidateIdent checks a database name.
func ValidateIdent(value, label string) error {
	if !ident.MatchString(value) {
		return fmt.Errorf("%s must contain only letters, numbers, underscores, and hyphens", label)
	}
	return nil
}

// ValidateUsername checks a database user name.
func ValidateUsername(value string) error {
	if !userName.MatchString(value) {
		return fmt.Errorf("username must contain only letters, numbers, dots, underscores, and hyphens")
	}
	return nil
}

// ValidatePort checks a TCP port.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// ValidateTLSMode checks a supported TLS mode. Empty defaults to require for network databases.
func ValidateTLSMode(mode string) (string, error) {
	if mode == "" {
		mode = "require"
	}
	switch mode {
	case "disable", "require", "verify-ca":
		return mode, nil
	default:
		return "", fmt.Errorf("tls_mode must be disable, require, or verify-ca")
	}
}

// ValidateParams checks the allowlisted extra connection parameters.
func ValidateParams(dbType string, params map[string]string) error {
	if len(params) == 0 {
		return nil
	}
	for key, value := range params {
		if strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("parameter %s contains unsupported characters", key)
		}
		switch dbType {
		case "postgres":
			switch key {
			case "connect_timeout":
				if !regexp.MustCompile(`^[1-9][0-9]{0,2}$`).MatchString(value) {
					return fmt.Errorf("connect_timeout must be a number of seconds from 1 to 999")
				}
			case "application_name":
				if !appName.MatchString(value) {
					return fmt.Errorf("application_name contains unsupported characters")
				}
			default:
				return fmt.Errorf("parameter %s is not allowed", key)
			}
		case "mysql":
			switch key {
			case "timeout", "readTimeout", "writeTimeout":
				if !duration.MatchString(value) {
					return fmt.Errorf("%s must look like 10s or 500ms", key)
				}
			case "charset":
				if !charset.MatchString(value) {
					return fmt.Errorf("charset contains unsupported characters")
				}
			default:
				return fmt.Errorf("parameter %s is not allowed", key)
			}
		default:
			return fmt.Errorf("parameters are not supported for this database")
		}
	}
	return nil
}

// ValidateCAPem checks an optional PEM certificate bundle.
func ValidateCAPem(pem string) error {
	if strings.TrimSpace(pem) == "" {
		return nil
	}
	if len(pem) > 64*1024 {
		return fmt.Errorf("tls ca bundle is too large")
	}
	if strings.Contains(pem, "\x00") || !strings.Contains(pem, "-----BEGIN CERTIFICATE-----") {
		return fmt.Errorf("tls ca bundle must be a PEM certificate")
	}
	return nil
}

// ValidateAdminUsername checks the single administrator account name.
func ValidateAdminUsername(name string) error {
	if !userName.MatchString(name) {
		return fmt.Errorf("username must contain only letters, numbers, dots, underscores, and hyphens")
	}
	return nil
}

// ValidateSourceName checks the label shown in the UI.
func ValidateSourceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return fmt.Errorf("name must be between 1 and 80 characters")
	}
	if strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("name contains unsupported characters")
	}
	return nil
}

// ValidateSQLitePath resolves symlinks and requires the file to stay under an allowed root.
func ValidateSQLitePath(path string, roots []string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("sqlite path must be absolute")
	}
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("sqlite path contains unsupported characters")
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("sqlite database file is not accessible")
	}
	resolved = filepath.Clean(resolved)
	for _, root := range roots {
		root = filepath.Clean(root)
		rootResolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		rootResolved = filepath.Clean(rootResolved)
		rel, err := filepath.Rel(rootResolved, resolved)
		if err != nil {
			continue
		}
		if rel == "." || strings.HasPrefix(rel, "..") || strings.Contains(rel, ".."+string(filepath.Separator)) {
			continue
		}
		// A file directly inside the root has no ".." and is not the root itself.
		if rel != ".." && !strings.HasPrefix(rel, "..") {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("sqlite path is outside the allowed directories")
}

// ValidateBucketName checks an S3 bucket name.
func ValidateBucketName(bucket string) error {
	if bucket == "" {
		return nil
	}
	if !bucketRE.MatchString(bucket) || strings.Contains(bucket, "..") {
		return fmt.Errorf("bucket name is invalid")
	}
	return nil
}

// ValidatePrefix checks an object key prefix.
func ValidatePrefix(prefix string) error {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return nil
	}
	if len(prefix) > 256 || strings.Contains(prefix, "\\") || strings.Contains(prefix, "..") || strings.ContainsAny(prefix, "\r\n\x00 ") {
		return fmt.Errorf("object prefix is invalid")
	}
	return nil
}

// Within reports whether child is parent or a directory inside parent.
func Within(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !strings.HasPrefix(rel, ".."))
}

// ValidateDataPaths rejects backup directories that overlap unsafe locations or the metadata database.
func ValidateDataPaths(backupDir, metaPath string) error {
	backupDir = filepath.Clean(backupDir)
	metaPath = filepath.Clean(metaPath)
	if !filepath.IsAbs(backupDir) || !filepath.IsAbs(metaPath) {
		return fmt.Errorf("backup and metadata paths must be absolute")
	}
	if backupDir == string(filepath.Separator) {
		return fmt.Errorf("backup directory is not allowed")
	}
	unsafe := []string{"/etc", "/proc", "/sys", "/dev", "/boot", "/root", "/usr", "/bin", "/sbin", "/lib", "/lib64", "/var/run", "/run"}
	for _, path := range unsafe {
		if Within(path, backupDir) {
			return fmt.Errorf("backup directory is not allowed")
		}
	}
	metaDir := filepath.Dir(metaPath)
	if Within(backupDir, metaPath) || Within(backupDir, metaDir) || Within(metaDir, backupDir) {
		return fmt.Errorf("backup directory must not overlap the metadata database")
	}
	return nil
}
