package netguard

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedIP(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "172.16.0.1": true, "192.168.1.1": true,
		"169.254.169.254": true, "0.0.0.0": true, "::": true, "::1": true, "fe80::1": true,
		"fd00::1": true, "::ffff:127.0.0.1": true, "::ffff:10.0.0.1": true, "::ffff:169.254.169.254": true,
		"100.64.0.1": true, "100.100.100.200": true, "224.0.0.1": true, "255.255.255.255": true,
		"93.184.216.34": false, "8.8.8.8": false, "2606:4700:4700::1111": false, "::ffff:8.8.8.8": false, "100.128.0.1": false,
	}
	for s, want := range tests {
		assert.Equal(t, want, IsBlockedIP(net.ParseIP(s)), s)
	}
	assert.True(t, IsBlockedIP(nil))
}

func TestDialContextRefusesInternalTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	for _, addr := range []string{srv.Listener.Addr().String(), "localhost:80", "0.0.0.0:80", "[::1]:80", "169.254.169.254:80", "[::ffff:127.0.0.1]:80"} {
		_, err := DialContext(context.Background(), "tcp", addr)
		assert.ErrorIs(t, err, ErrPrivateAddress, addr)
	}
	_, err := DialContext(context.Background(), "tcp", "no-port")
	require.Error(t, err)
	_, err = DialContext(context.Background(), "tcp", "nonexistent.invalid:80")
	assert.ErrorIs(t, err, ErrUnreachable)
}
