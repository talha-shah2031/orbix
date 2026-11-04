package endpoints

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryIsGETOnlyAndReadOnly(t *testing.T) {
	for _, definition := range Allowed() {
		require.Equal(t, http.MethodGet, definition.Method)
		lower := strings.ToLower(definition.Path)
		for _, forbidden := range []string{"/cancel", "withdraw", "transfer", "borrow", "repay", "listenkey"} {
			require.NotContains(t, lower, forbidden, "unexpected mutable or user-stream route: %s", definition.Path)
		}
		require.NotEqual(t, "/api/v3/order", lower)
		require.NotEqual(t, "/fapi/v1/order", lower)
		require.NotEqual(t, "/dapi/v1/order", lower)
	}
}

func TestWriteAndUnknownRoutesRejected(t *testing.T) {
	for _, test := range []struct{ method, path string }{
		{http.MethodPost, "/api/v3/order"},
		{http.MethodDelete, "/api/v3/order"},
		{http.MethodPut, "/api/v3/order/amend/keepPriority"},
		{http.MethodPost, "/sapi/v1/capital/withdraw/apply"},
		{http.MethodPost, "/sapi/v1/asset/transfer"},
		{http.MethodPost, "/sapi/v1/margin/borrow-repay"},
		{http.MethodGet, "/api/v3/order"},
	} {
		_, err := Lookup(Spot, test.method, test.path)
		require.ErrorIs(t, err, ErrEndpointNotAllowed, "%s %s", test.method, test.path)
	}
}

func TestReadOnlyPrivateRoutesRequireSignature(t *testing.T) {
	definition, err := Lookup(Spot, http.MethodGet, "/api/v3/account")
	require.NoError(t, err)
	require.True(t, definition.Signed)
	definition, err = Lookup(Spot, http.MethodGet, "/api/v3/historicalTrades")
	require.NoError(t, err)
	require.True(t, definition.APIKey)
	require.False(t, definition.Signed)
}
