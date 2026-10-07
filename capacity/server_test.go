package capacity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testToken = "synthetic-reader-token-32-characters-long"

func snapshotForTest() Snapshot {
	now := time.Now().UTC()
	return Snapshot{Version: Version, GeneratedAt: now, ExpiresAt: now.Add(10 * time.Minute), Accounts: []Account{{ID: "synthetic-account", CredentialIndexes: []string{"opaque-index"}, Provider: "test", Status: "ok", ObservedAt: now, ExpiresAt: now.Add(10 * time.Minute), Source: "test", AdapterVersion: "1", Report: Report{Groups: []Group{{Buckets: []Bucket{{Scope: "account", Window: "weekly", RemainingFraction: .5}}}}}}}}
}
func TestPrivateEndpointAuthenticationAndSeparation(t *testing.T) {
	s, err := NewServer(testToken, func(context.Context) (Snapshot, error) { return snapshotForTest(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, token string
		want        int
	}{{"/v1/allowances", "", 401}, {"/v1/allowances", "incorrect", 401}, {"/v1/allowances", testToken, 200}, {"/catalog.json", testToken, 404}, {"/v1/catalog", testToken, 404}, {"/healthz", "", 200}} {
		r := httptest.NewRequest("GET", test.path, nil)
		if test.token != "" {
			r.Header.Set("Authorization", "Bearer "+test.token)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("%s: %d", test.path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private response caching allowed")
		}
	}
}
func TestExpiredSnapshotAndRefreshFailure(t *testing.T) {
	snapshot := snapshotForTest()
	fail := false
	s, _ := NewServer(testToken, func(context.Context) (Snapshot, error) {
		if fail {
			return Snapshot{}, errors.New("synthetic source failure")
		}
		return snapshot, nil
	})
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := s.snapshot.ExpiresAt
	fail = true
	if err := s.Refresh(context.Background()); err == nil {
		t.Fatal("refresh unexpectedly succeeded")
	}
	if s.snapshot.ExpiresAt != before {
		t.Fatal("failure extended observation lifetime")
	}
	s.snapshot.ExpiresAt = time.Now().Add(-time.Second)
	r := httptest.NewRequest("GET", "/v1/allowances", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestUnsafeServerConfiguration(t *testing.T) {
	if _, err := NewServer("", func(context.Context) (Snapshot, error) { return snapshotForTest(), nil }); err == nil {
		t.Fatal("empty token accepted")
	}
	if _, err := NewServer(testToken, nil); err == nil {
		t.Fatal("nil collector accepted")
	}
}
func TestSnapshotValidation(t *testing.T) {
	snapshot := snapshotForTest()
	if err := snapshot.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	snapshot.Accounts[0].Report.Groups[0].Buckets[0].Scope = "model"
	if err := snapshot.Validate(time.Now()); err == nil {
		t.Fatal("unscoped model allowance accepted")
	}
	snapshot.Accounts[0].Report.Groups[0].Buckets[0].Models = []string{"model"}
	if err := snapshot.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	snapshot.Accounts[0].Report.Groups[0].Buckets[0].RemainingFraction = 1.1
	if err := snapshot.Validate(time.Now()); err == nil {
		t.Fatal("invalid remaining fraction accepted")
	}
}
func TestCollectorRedirectRejected(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("collector redirect followed") }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer source.Close()
	if _, err := Fetch(context.Background(), source.Client(), source.URL, "synthetic"); err == nil {
		t.Fatal("redirect accepted")
	}
}
