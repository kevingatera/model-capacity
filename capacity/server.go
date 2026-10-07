package capacity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu          sync.RWMutex
	snapshot    *Snapshot
	readerToken [32]byte
	fetch       func(context.Context) (Snapshot, error)
}

func NewServer(readerToken string, fetch func(context.Context) (Snapshot, error)) (*Server, error) {
	if len(readerToken) < 32 || fetch == nil {
		return nil, fmt.Errorf("a strong reader token and collector are required")
	}
	return &Server{readerToken: sha256.Sum256([]byte(readerToken)), fetch: fetch}, nil
}
func (s *Server) Refresh(ctx context.Context) error {
	snapshot, err := s.fetch(ctx)
	if err != nil {
		return err
	}
	if err = snapshot.Validate(time.Now()); err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshot = &snapshot
	s.mu.Unlock()
	return nil
}
func (s *Server) Run(ctx context.Context, interval time.Duration) {
	if interval < time.Minute {
		interval = time.Minute
	}
	refresh := func() {
		if err := s.Refresh(ctx); err != nil {
			log.Print("allowance refresh failed; previous snapshot retains its original expiry")
		}
	}
	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/healthz" {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	if r.URL.Path != "/v1/allowances" {
		http.NotFound(w, r)
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	candidate := sha256.Sum256([]byte(strings.TrimPrefix(header, "Bearer ")))
	if subtle.ConstantTimeCompare(candidate[:], s.readerToken[:]) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s.mu.RLock()
	snapshot := s.snapshot
	s.mu.RUnlock()
	if snapshot == nil || !snapshot.ExpiresAt.After(time.Now()) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"allowance_snapshot_unavailable"}`))
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}
