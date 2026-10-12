//go:build !darwin

package conn

// NewSecretStore returns the platform secret store: a 0600 JSON file under
// dir on non-macOS platforms (see ADR 0004).
func NewSecretStore(dir string) SecretStore {
	s, err := NewFileSecretStore(dir)
	if err != nil {
		return &erroringSecretStore{err: err}
	}
	return s
}

type erroringSecretStore struct{ err error }

func (e *erroringSecretStore) Set(account, secret string) error { return e.err }
func (e *erroringSecretStore) Get(account string) string        { return "" }
func (e *erroringSecretStore) Delete(account string) error      { return e.err }
