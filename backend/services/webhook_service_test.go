package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func webhookDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.Webhook{}, &models.WebhookDelivery{}))
	return db
}

func TestComputeSignatureIsHMACSHA256Hex(t *testing.T) {
	body := []byte(`{"a":1}`)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(body)
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), computeSignature("s3cret", body))
	assert.NotEqual(t, computeSignature("s3cret", body), computeSignature("other", body))
}

func TestDeliverWebhookSignsAndRecordsSuccess(t *testing.T) {
	db := webhookDB(t)
	var gotSig, gotEvent, gotType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig, gotEvent, gotType = r.Header.Get("X-Webhook-Signature"), r.Header.Get("X-Meerkat-Event"), r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	wh := models.Webhook{UserID: 1, URL: srv.URL, Secret: "s3cret"}
	require.NoError(t, db.Create(&wh).Error)
	body := []byte(`{"event":"contact.created"}`)
	d := deliverWebhook(db, config.Config{}, wh, "contact.created", body, 1)

	assert.Equal(t, "sha256="+computeSignature("s3cret", body), gotSig)
	assert.Equal(t, "contact.created", gotEvent)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, body, gotBody)
	require.NotNil(t, d.StatusCode)
	assert.Equal(t, 204, *d.StatusCode)
	assert.Nil(t, d.Error)
	assert.Nil(t, d.NextRetryAt)
}

func TestDeliverWebhookRetrySchedule(t *testing.T) {
	db := webhookDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	wh := models.Webhook{UserID: 1, URL: srv.URL, Secret: "s"}
	require.NoError(t, db.Create(&wh).Error)

	d1 := deliverWebhook(db, config.Config{}, wh, "e", []byte("{}"), 1)
	require.NotNil(t, d1.NextRetryAt)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), *d1.NextRetryAt, time.Minute)
	assert.NotNil(t, d1.Error)

	d2 := deliverWebhook(db, config.Config{}, wh, "e", []byte("{}"), 2)
	require.NotNil(t, d2.NextRetryAt)
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), *d2.NextRetryAt, time.Minute)

	d3 := deliverWebhook(db, config.Config{}, wh, "e", []byte("{}"), 3)
	assert.Nil(t, d3.NextRetryAt, "no retry after the last attempt")
}

func TestDeliverWebhookConnectionErrorSchedulesRetry(t *testing.T) {
	db := webhookDB(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	l.Close()
	wh := models.Webhook{UserID: 1, URL: "http://" + addr, Secret: "s"}
	d := deliverWebhook(db, config.Config{}, wh, "e", []byte("{}"), 1)
	assert.NotNil(t, d.Error)
	assert.Nil(t, d.StatusCode)
	assert.NotNil(t, d.NextRetryAt)
}

func TestIsPrivateURL(t *testing.T) {
	for _, u := range []string{
		"http://127.0.0.1/x", "http://[::1]/x", "http://10.1.2.3/x", "http://192.168.0.1/",
		"http://169.254.169.254/latest/meta-data", "http://0.0.0.0:8080/", "http://[::ffff:127.0.0.1]/", "http://localhost/", "://bad",
	} {
		assert.True(t, isPrivateURL(u), u)
	}
	assert.False(t, isPrivateURL("http://8.8.8.8/hook"))
}

func TestDeliverWebhookBlocksPrivateTargets(t *testing.T) {
	db := webhookDB(t)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	wh := models.Webhook{UserID: 1, URL: srv.URL, Secret: "s"}

	d := deliverWebhook(db, config.Config{WebhookBlockPrivateURLs: true}, wh, "e", []byte("{}"), 1)
	require.NotNil(t, d.Error)
	assert.Contains(t, *d.Error, "private")
	assert.Nil(t, d.NextRetryAt, "a blocked target is not retried")
	assert.Zero(t, atomic.LoadInt32(&hits))
}

// The pre-check looks at the URL only. The delivery client must also refuse at connect time, which
// covers redirects to internal hosts and DNS rebinding.
func TestBlockingClientRefusesLoopbackAtDialTime(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	resp, err := blockingDeliveryClient().Get(srv.URL) //nolint:bodyclose // no response on error
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Error(t, err)
	assert.Zero(t, atomic.LoadInt32(&hits))

	// The unspecified address reaches the local host on Linux and is refused too.
	resp, err = blockingDeliveryClient().Get("http://0.0.0.0:1/") //nolint:bodyclose // no response on error
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Error(t, err)
}

func TestTriggerWebhooksFiltersByUserEventAndActive(t *testing.T) {
	db := webhookDB(t)
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
	}))
	defer srv.Close()
	mk := func(path string, user uint, events []string, active bool) {
		wh := models.Webhook{UserID: user, Name: path, URL: srv.URL + path, Secret: "s", Events: events, IsActive: true}
		require.NoError(t, db.Create(&wh).Error)
		if !active {
			require.NoError(t, db.Model(&wh).Update("is_active", false).Error)
		}
	}
	mk("/match", 1, []string{"contact.created"}, true)
	mk("/other-event", 1, []string{"contact.deleted"}, true)
	mk("/inactive", 1, []string{"contact.created"}, false)
	mk("/other-user", 2, []string{"contact.created"}, true)

	TriggerWebhooks(db, config.Config{}, 1, "contact.created", map[string]string{"k": "v"})
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return hits["/match"] == 1 }, 5*time.Second, 20*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, map[string]int{"/match": 1}, hits)
}

func TestProcessWebhookRetriesReplaysPayload(t *testing.T) {
	db := webhookDB(t)
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.Store(string(b))
	}))
	defer srv.Close()
	wh := models.Webhook{UserID: 1, URL: srv.URL, Secret: "s", IsActive: true}
	require.NoError(t, db.Create(&wh).Error)
	past := time.Now().Add(-time.Minute)
	failed := models.WebhookDelivery{WebhookID: wh.ID, EventType: "e", Payload: `{"id":"orig"}`, Attempts: 1, NextRetryAt: &past}
	require.NoError(t, db.Create(&failed).Error)
	exhausted := models.WebhookDelivery{WebhookID: wh.ID, EventType: "e", Payload: `{"id":"x"}`, Attempts: maxDeliveryAttempts, NextRetryAt: &past}
	require.NoError(t, db.Create(&exhausted).Error)

	ProcessWebhookRetries(db, config.Config{})

	require.Eventually(t, func() bool { return got.Load() != nil }, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, `{"id":"orig"}`, got.Load())
	require.Eventually(t, func() bool {
		var n int64
		db.Model(&models.WebhookDelivery{}).Where("attempts = 2").Count(&n)
		return n == 1
	}, 5*time.Second, 20*time.Millisecond)
	var reloaded models.WebhookDelivery
	require.NoError(t, db.First(&reloaded, failed.ID).Error)
	assert.Nil(t, reloaded.NextRetryAt, "the old delivery is not picked up again")
}

func TestProcessWebhookRetriesSkipsInactiveWebhook(t *testing.T) {
	db := webhookDB(t)
	wh := models.Webhook{UserID: 1, URL: "http://127.0.0.1:1", Secret: "s", IsActive: true}
	require.NoError(t, db.Create(&wh).Error)
	require.NoError(t, db.Model(&wh).Update("is_active", false).Error)
	past := time.Now().Add(-time.Minute)
	d := models.WebhookDelivery{WebhookID: wh.ID, EventType: "e", Payload: "{}", Attempts: 1, NextRetryAt: &past}
	require.NoError(t, db.Create(&d).Error)

	ProcessWebhookRetries(db, config.Config{})

	var n int64
	db.Model(&models.WebhookDelivery{}).Count(&n)
	assert.Equal(t, int64(1), n, "no new delivery")
	var reloaded models.WebhookDelivery
	require.NoError(t, db.First(&reloaded, d.ID).Error)
	assert.Nil(t, reloaded.NextRetryAt)
}
