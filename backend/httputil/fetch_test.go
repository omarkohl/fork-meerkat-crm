package httputil

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Photo URLs come from users and vCards: internal targets are always refused, whatever
// the BLOCK_PRIVATE_URLS setting.
func TestFetchImageFromURLRefusesInternalTargets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
	}))
	defer srv.Close()
	for _, u := range []string{srv.URL + "/a.png", "http://localhost/a.png", "http://0.0.0.0/a.png", "http://[::ffff:127.0.0.1]/a.png", "http://169.254.169.254/latest/meta-data", "http://100.100.100.200/"} {
		_, _, err := FetchImageFromURL(u)
		require.Error(t, err, u)
	}
	assert.Zero(t, hits.Load())
}
