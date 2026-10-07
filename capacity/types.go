package capacity

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const Version = 1

type Snapshot struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Accounts    []Account `json:"accounts"`
}
type Account struct {
	ID                string    `json:"id"`
	CredentialIndexes []string  `json:"credential_indexes"`
	Provider          string    `json:"provider"`
	Plan              string    `json:"plan,omitempty"`
	Status            string    `json:"status"`
	ObservedAt        time.Time `json:"observed_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	Source            string    `json:"source"`
	AdapterVersion    string    `json:"adapter_version"`
	Report            Report    `json:"report"`
	Models            []Model   `json:"models,omitempty"`
}
type Model struct {
	ID            string `json:"id"`
	ContextLength int    `json:"context_length,omitempty"`
}
type Report struct {
	Groups  []Group  `json:"groups,omitempty"`
	Summary []Metric `json:"summary,omitempty"`
}
type Group struct {
	DisplayName string   `json:"displayName,omitempty"`
	Buckets     []Bucket `json:"buckets,omitempty"`
}
type Bucket struct {
	Scope             string   `json:"scope"`
	Models            []string `json:"models,omitempty"`
	Window            string   `json:"window"`
	RemainingFraction float64  `json:"remainingFraction"`
	ResetTime         string   `json:"resetTime,omitempty"`
}
type Metric struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit,omitempty"`
	Format   string  `json:"format,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

func (s Snapshot) Validate(now time.Time) error {
	if s.Version != Version {
		return fmt.Errorf("unsupported snapshot version")
	}
	if s.GeneratedAt.IsZero() || s.GeneratedAt.After(now.Add(time.Minute)) || !s.ExpiresAt.After(s.GeneratedAt) || s.ExpiresAt.Sub(s.GeneratedAt) > 30*time.Minute {
		return fmt.Errorf("invalid snapshot lifetime")
	}
	if len(s.Accounts) > 1000 {
		return fmt.Errorf("too many accounts")
	}
	seen := map[string]bool{}
	for _, account := range s.Accounts {
		if account.ID == "" || strings.Contains(account.ID, "@") || seen[account.ID] {
			return fmt.Errorf("invalid or duplicate account identifier")
		}
		seen[account.ID] = true
		if account.Provider == "" || account.Source == "" || account.AdapterVersion == "" {
			return fmt.Errorf("account provenance is required")
		}
		if account.Status != "ok" && account.Status != "unavailable" {
			return fmt.Errorf("invalid account status")
		}
		if account.ObservedAt.IsZero() || account.ObservedAt.After(now.Add(time.Minute)) || !account.ExpiresAt.After(account.ObservedAt) || account.ExpiresAt.Sub(account.ObservedAt) > 30*time.Minute {
			return fmt.Errorf("invalid account lifetime")
		}
		if account.Status == "unavailable" && (len(account.Report.Groups) != 0 || len(account.Report.Summary) != 0) {
			return fmt.Errorf("unavailable accounts must not advertise allowance")
		}
		for _, group := range account.Report.Groups {
			for _, bucket := range group.Buckets {
				if bucket.Window == "" {
					return fmt.Errorf("allowance window is required")
				}
				if bucket.Scope != "account" && bucket.Scope != "model" {
					return fmt.Errorf("invalid allowance scope")
				}
				if bucket.Scope == "model" && len(bucket.Models) == 0 {
					return fmt.Errorf("model allowance needs model identifiers")
				}
				if math.IsNaN(bucket.RemainingFraction) || math.IsInf(bucket.RemainingFraction, 0) || bucket.RemainingFraction < 0 || bucket.RemainingFraction > 1 {
					return fmt.Errorf("invalid remaining fraction")
				}
				if bucket.ResetTime != "" {
					if _, err := time.Parse(time.RFC3339Nano, bucket.ResetTime); err != nil {
						return fmt.Errorf("invalid reset time")
					}
				}
			}
		}
		for _, metric := range account.Report.Summary {
			if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
				return fmt.Errorf("invalid account metric")
			}
		}
	}
	return nil
}
