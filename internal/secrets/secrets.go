package secrets

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const service = "orbix"

var ErrNotFound = errors.New("secret not found")

type Credentials struct{ APIKey, APISecret string }

type Store interface {
	Get(profile string) (Credentials, error)
	Set(profile string, credentials Credentials) error
	Delete(profile string) error
}

// FallbackStore uses a primary OS-backed store and a read-only fallback such as environment variables.
type FallbackStore struct{ Primary, Fallback Store }

func (s FallbackStore) Get(profile string) (Credentials, error) {
	if s.Primary != nil {
		credentials, err := s.Primary.Get(profile)
		if err == nil {
			return credentials, nil
		}
		if !errors.Is(err, ErrNotFound) {
			if s.Fallback == nil {
				return Credentials{}, err
			}
		}
	}
	if s.Fallback != nil {
		return s.Fallback.Get(profile)
	}
	return Credentials{}, ErrNotFound
}
func (s FallbackStore) Set(profile string, credentials Credentials) error {
	if s.Primary == nil {
		return errors.New("no writable secret store configured")
	}
	return s.Primary.Set(profile, credentials)
}
func (s FallbackStore) Delete(profile string) error {
	if s.Primary == nil {
		return errors.New("no writable secret store configured")
	}
	return s.Primary.Delete(profile)
}

type KeyringStore struct{}

func (KeyringStore) Get(profile string) (Credentials, error) {
	key, err := keyring.Get(service, profile+":api-key")
	if err != nil {
		return Credentials{}, normalize(err)
	}
	secret, err := keyring.Get(service, profile+":api-secret")
	if err != nil {
		return Credentials{}, normalize(err)
	}
	return Credentials{APIKey: key, APISecret: secret}, nil
}

func (KeyringStore) Set(profile string, credentials Credentials) error {
	if credentials.APIKey == "" || credentials.APISecret == "" {
		return errors.New("API key and secret are required")
	}
	if err := keyring.Set(service, profile+":api-key", credentials.APIKey); err != nil {
		return fmt.Errorf("store API key: %w", err)
	}
	if err := keyring.Set(service, profile+":api-secret", credentials.APISecret); err != nil {
		return fmt.Errorf("store API secret: %w", err)
	}
	return nil
}

func (KeyringStore) Delete(profile string) error {
	keyErr := normalize(keyring.Delete(service, profile+":api-key"))
	secretErr := normalize(keyring.Delete(service, profile+":api-secret"))
	if keyErr != nil && !errors.Is(keyErr, ErrNotFound) {
		return keyErr
	}
	if secretErr != nil && !errors.Is(secretErr, ErrNotFound) {
		return secretErr
	}
	return nil
}

type EnvStore struct{}

func (EnvStore) Get(string) (Credentials, error) {
	credentials := Credentials{APIKey: os.Getenv("ORBIX_API_KEY"), APISecret: os.Getenv("ORBIX_API_SECRET")}
	if credentials.APIKey == "" || credentials.APISecret == "" {
		return Credentials{}, ErrNotFound
	}
	return credentials, nil
}
func (EnvStore) Set(string, Credentials) error {
	return errors.New("environment secret store is read-only")
}
func (EnvStore) Delete(string) error { return errors.New("environment secret store is read-only") }

func normalize(err error) error {
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
