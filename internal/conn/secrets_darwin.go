//go:build darwin

package conn

// NewSecretStore returns the platform secret store: the macOS Keychain via
// the security CLI (see ADR 0004).
func NewSecretStore(dir string) SecretStore { return NewKeychainStore() }
