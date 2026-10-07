package capacity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Fetch reads a bounded, versioned snapshot. Error bodies are never returned to callers.
func Fetch(ctx context.Context, client *http.Client, endpoint, token string) (Snapshot, error) {
	var snapshot Snapshot
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return snapshot, fmt.Errorf("invalid allowance endpoint")
	}
	if client == nil {
		client = &http.Client{}
	}
	localClient := *client
	localClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return snapshot, fmt.Errorf("invalid allowance endpoint")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := localClient.Do(request)
	if err != nil {
		return snapshot, fmt.Errorf("allowance source request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return snapshot, fmt.Errorf("allowance source returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return snapshot, fmt.Errorf("invalid allowance response size")
	}
	if err = json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, fmt.Errorf("invalid allowance JSON")
	}
	if err = snapshot.Validate(time.Now()); err != nil {
		return snapshot, err
	}
	if !snapshot.ExpiresAt.After(time.Now()) {
		return snapshot, fmt.Errorf("allowance snapshot expired")
	}
	return snapshot, nil
}
