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
