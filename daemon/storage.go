package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TokenSnapshot is a point-in-time record of token usage.
type TokenSnapshot struct {
	Timestamp   time.Time        `json:"timestamp"`
	Window      *WindowInfo      `json:"window"`
	APIUsage    *APIUsageInfo    `json:"apiUsage"`
	LocalTokens *LocalTokens     `json:"localTokens"`
}

// WindowInfo describes the API 5h window boundaries.
type WindowInfo struct {
	StartsAt string `json:"startsAt"`
	ResetsAt string `json:"resetsAt"`
}

// APIUsageInfo holds the API utilization percentages.
type APIUsageInfo struct {
	FiveHourUtilization       float64 `json:"fiveHourUtilization"`
	SevenDayUtilization       float64 `json:"sevenDayUtilization"`
	SevenDaySonnetUtilization float64 `json:"sevenDaySonnetUtilization"`
}

// TokenUsageFile is the on-disk format for token-usage.json.
type TokenUsageFile struct {
	Hostname  string          `json:"hostname"`
	Snapshots []TokenSnapshot `json:"snapshots"`
}

const maxSnapshotAge = 7 * 24 * time.Hour

// buildAPIUsageInfo extracts utilization percentages from the API response.
func buildAPIUsageInfo(usage *UsageResponse) *APIUsageInfo {
	info := &APIUsageInfo{}
	if usage.FiveHour != nil {
		info.FiveHourUtilization = usage.FiveHour.Utilization
	}
	if usage.SevenDay != nil {
		info.SevenDayUtilization = usage.SevenDay.Utilization
	}
	if usage.SevenDaySonnet != nil {
		info.SevenDaySonnetUtilization = usage.SevenDaySonnet.Utilization
	}
	return info
}

func tokenUsagePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tokeneater", "token-usage.json"), nil
}

// loadTokenUsage reads the token usage file, or returns an empty struct if missing.
func loadTokenUsage() (*TokenUsageFile, error) {
	path, err := tokenUsagePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		hostname, _ := os.Hostname()
		return &TokenUsageFile{Hostname: hostname}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading token usage: %w", err)
	}

	var f TokenUsageFile
	if err := json.Unmarshal(data, &f); err != nil {
		hostname, _ := os.Hostname()
		return &TokenUsageFile{Hostname: hostname}, nil
	}
	return &f, nil
}

// appendSnapshot adds a snapshot and prunes old entries, then writes to disk.
func appendSnapshot(snap TokenSnapshot) error {
	f, err := loadTokenUsage()
	if err != nil {
		return err
	}

	f.Snapshots = append(f.Snapshots, snap)

	// Prune old snapshots
	cutoff := time.Now().Add(-maxSnapshotAge)
	kept := f.Snapshots[:0]
	for _, s := range f.Snapshots {
		if s.Timestamp.After(cutoff) {
			kept = append(kept, s)
		}
	}
	f.Snapshots = kept

	path, err := tokenUsagePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating dir: %w", err)
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
