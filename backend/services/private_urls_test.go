package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every outbound path that honours the private-URL switch must refuse a loopback server
// when blocking is on and reach it when it is off. The httptest server stands for an
// internal host.
func TestPrivateURLSwitchOnCalendarAndCardDAVClients(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, tc := range []struct {
		name string
		call func(block bool) error
		want error
	}{
		{"calendar", func(block bool) error {
			resp, err := NewCalendarSyncService(block).client.Get(srv.URL)
			if resp != nil {
				_ = resp.Body.Close()
			}
			return err
		}, ErrCalendarPrivateAddress},
		{"carddav", func(block bool) error {
			resp, err := NewContactSyncService(block).client.Get(srv.URL)
			if resp != nil {
				_ = resp.Body.Close()
			}
			return err
		}, ErrContactSyncPrivateAddress},
		{"carddav discovery (well-known)", func(block bool) error {
			_, err := NewContactSyncService(block).Discover(ctx, srv.URL, "u", "p")
			return err
		}, ErrContactSyncPrivateAddress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits.Store(0)
			err := tc.call(true)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
			assert.Zero(t, hits.Load(), "a blocked request must not reach the server")

			// Blocking off: the request reaches the server (discovery may still fail on the protocol).
			_ = tc.call(false)
			assert.NotZero(t, hits.Load())
		})
	}
}
