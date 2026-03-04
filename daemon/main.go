package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const refreshInterval = 5 * time.Minute

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("tokeneater-daemon starting")

	credPath, err := defaultCredentialsPath()
	if err != nil {
		log.Fatalf("resolving credentials path: %v", err)
	}

	apiClient := newAPIClient()
	notif := newNotifier()
	refreshCh := make(chan struct{}, 1)

	// Retry D-Bus connection — the session bus may not be ready yet at boot
	var dbus *dbusServer
	for attempt := 1; attempt <= 5; attempt++ {
		dbus, err = newDBusServer()
		if err == nil {
			break
		}
		log.Printf("D-Bus server attempt %d/5 failed: %v", attempt, err)
		if attempt < 5 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	if err != nil {
		log.Fatalf("D-Bus server: %v (gave up after 5 attempts)", err)
	}
	defer dbus.close()
	dbus.setRefreshCh(refreshCh)

	fetch := func() {
		log.Println("fetching usage...")
		token, err := readToken(credPath)
		if err != nil {
			log.Printf("token read error: %v", err)
			s := buildState(nil, nil, err)
			dbus.emitStateChanged(s.JSON())
			return
		}

		usage, err := apiClient.fetchUsage(token)
		if err != nil {
			log.Printf("API error: %v", err)
			s := buildState(nil, nil, err)
			dbus.emitStateChanged(s.JSON())
			return
		}

		var pacing *PacingResult
		if usage.SevenDay != nil {
			p, err := calculatePacing(usage.SevenDay)
			if err != nil {
				log.Printf("pacing error: %v", err)
			} else {
				pacing = p
			}
		}

		notif.CheckThresholds(usage)

		s := buildState(usage, pacing, nil)

		// Track local token usage aligned to the 5h window
		if usage.FiveHour != nil && usage.FiveHour.ResetsAt != "" {
			resetsAt, err := usage.FiveHour.ResetsAtTime()
			if err == nil {
				windowStart := resetsAt.Add(-5 * time.Hour)
				localTokens, err := scanLocalTokens(windowStart, resetsAt)
				if err != nil {
					log.Printf("token tracker error: %v", err)
				} else {
					s.TokenUsage = &TokenUsageData{
						InputTokens:         localTokens.InputTokens,
						OutputTokens:        localTokens.OutputTokens,
						CacheCreationTokens: localTokens.CacheCreationTokens,
						CacheReadTokens:     localTokens.CacheReadTokens,
						TotalTokens:         localTokens.TotalTokens,
						WindowMinutes:       300,
					}
					log.Printf("local tokens (5h window): total=%d (in=%d out=%d cache_create=%d cache_read=%d)",
						localTokens.TotalTokens, localTokens.InputTokens, localTokens.OutputTokens,
						localTokens.CacheCreationTokens, localTokens.CacheReadTokens)

					// Store snapshot
					snap := TokenSnapshot{
						Timestamp: time.Now(),
						Window: &WindowInfo{
							StartsAt: windowStart.Format(time.RFC3339),
							ResetsAt: usage.FiveHour.ResetsAt,
						},
						APIUsage:    buildAPIUsageInfo(usage),
						LocalTokens: localTokens,
					}
					if err := appendSnapshot(snap); err != nil {
						log.Printf("snapshot storage error: %v", err)
					}
				}
			}
		}

		dbus.emitStateChanged(s.JSON())

		sessionPct := 0.0
		if usage.FiveHour != nil {
			sessionPct = usage.FiveHour.Utilization
		}
		weeklyPct := 0.0
		if usage.SevenDay != nil {
			weeklyPct = usage.SevenDay.Utilization
		}
		log.Printf("state updated: session=%.0f%% weekly=%.0f%%", sessionPct, weeklyPct)
	}

	fetch()

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case <-ticker.C:
			fetch()
		case <-refreshCh:
			log.Println("manual refresh requested")
			fetch()
		case sig := <-sigCh:
			log.Printf("received %v, shutting down", sig)
			return
		}
	}
}
