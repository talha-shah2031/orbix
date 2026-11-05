package secrets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvStore(t *testing.T) {
	t.Setenv("ORBIX_API_KEY", "key")
	t.Setenv("ORBIX_API_SECRET", "secret")
	credentials, err := (EnvStore{}).Get("main")
	require.NoError(t, err)
	require.Equal(t, "key", credentials.APIKey)
}

func TestEnvStoreMissing(t *testing.T) {
	t.Setenv("ORBIX_API_KEY", "")
	t.Setenv("ORBIX_API_SECRET", "")
	_, err := (EnvStore{}).Get("main")
	require.ErrorIs(t, err, ErrNotFound)
}
