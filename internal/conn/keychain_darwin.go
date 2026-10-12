//go:build darwin

package conn

import (
	"os/exec"
	"strings"
)

// keychainService is the macOS Keychain item service name.
const keychainService = "dev.bernylinville.seismic"

// KeychainStore implements SecretStore via the macOS `security` CLI.
// account is the connection UUID; add-generic-password -U makes Set
// idempotent.
type KeychainStore struct{}

// NewKeychainStore returns the macOS Keychain-backed SecretStore.
func NewKeychainStore() *KeychainStore { return &KeychainStore{} }

// Set stores (or replaces, via -U) the generic-password item.
func (KeychainStore) Set(account, secret string) error {
	return exec.Command("security", "add-generic-password",
		"-U", "-s", keychainService, "-a", account, "-w", secret).Run()
}

// Get returns the stored secret; a missing item is "", not an error.
func (KeychainStore) Get(account string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-s", keychainService, "-a", account, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// Delete removes the item; a missing item is not an error.
func (KeychainStore) Delete(account string) error {
	err := exec.Command("security", "delete-generic-password",
		"-s", keychainService, "-a", account).Run()
	if err != nil && strings.Contains(err.Error(), "could not be found") {
		return nil
	}
	return err
}
