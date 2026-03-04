package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalTokens holds the aggregated token counts from local Claude sessions.
type LocalTokens struct {
	InputTokens         int64 `json:"inputTokens"`
	OutputTokens        int64 `json:"outputTokens"`
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
	CacheReadTokens     int64 `json:"cacheReadTokens"`
	TotalTokens         int64 `json:"totalTokens"`
}

// sessionLine represents a single line from a Claude session JSONL file.
type sessionLine struct {
	Timestamp string `json:"timestamp"`
	Data      *struct {
		Usage *usageBlock `json:"usage"`
	} `json:"data"`
	// Top-level usage (some formats)
	Usage *usageBlock `json:"usage"`
}

type usageBlock struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// scanLocalTokens scans ~/.claude/projects/ for token usage within the given time window.
func scanLocalTokens(windowStart, windowEnd time.Time) (*LocalTokens, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home: %w", err)
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return &LocalTokens{}, nil
	}

	cutoff := time.Now().Add(-6 * time.Hour)
	tokens := &LocalTokens{}

	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		// Skip files not modified recently
		if info.ModTime().Before(cutoff) {
			return nil
		}
		scanFile(path, windowStart, windowEnd, tokens)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking projects dir: %w", err)
	}

	tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens +
		tokens.CacheCreationTokens + tokens.CacheReadTokens

	return tokens, nil
}

func scanFile(path string, windowStart, windowEnd time.Time, tokens *LocalTokens) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Quick check: skip lines without "usage"
		if !strings.Contains(string(line), `"usage"`) {
			continue
		}

		var sl sessionLine
		if err := json.Unmarshal(line, &sl); err != nil {
			continue
		}

		// Parse timestamp
		if sl.Timestamp == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, sl.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, sl.Timestamp)
			if err != nil {
				continue
			}
		}

		if ts.Before(windowStart) || ts.After(windowEnd) {
			continue
		}

		// Extract usage block
		var u *usageBlock
		if sl.Data != nil && sl.Data.Usage != nil {
			u = sl.Data.Usage
		} else if sl.Usage != nil {
			u = sl.Usage
		}
		if u == nil {
			continue
		}

		tokens.InputTokens += u.InputTokens
		tokens.OutputTokens += u.OutputTokens
		tokens.CacheCreationTokens += u.CacheCreationInputTokens
		tokens.CacheReadTokens += u.CacheReadInputTokens
	}
}
