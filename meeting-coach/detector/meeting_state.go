package detector

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/your-username/meeting-coach/config"
	"github.com/your-username/meeting-coach/logs"
	"github.com/your-username/meeting-coach/models"
	"github.com/your-username/meeting-coach/report"
	"github.com/your-username/meeting-coach/screenpipe"
	"github.com/your-username/meeting-coach/speaker"
)

type MeetingDetector struct {
	state          models.MeetingState
	network        *NetworkMonitor
	spClient       *screenpipe.Client
	speakerTracker *speaker.Tracker
	reportGen      *report.Generator
}

func NewMeetingDetector() *MeetingDetector {
	return &MeetingDetector{
		state: models.MeetingState{
			Phase: models.PhaseIdle,
		},
		network:        NewNetworkMonitor(),
		spClient:       screenpipe.NewClient(),
		speakerTracker: speaker.NewTracker(),
		reportGen:      report.NewGenerator(),
	}
}

func (md *MeetingDetector) Start() {
	log.Println("Meeting Coach started!")
	log.Println("  Screenpipe API:", config.ScreenpipeBaseURL)
	log.Printf("  Stability window: %d sec\n", config.StabilityWindowSec)
	log.Printf("  Active check interval: %v\n", config.ActiveCheckInterval)
	log.Printf("  Failure threshold: %d\n", config.FailureThreshold)
	log.Println("  Waiting for meeting app...")
	log.Println()
	md.runIdleLoop()
}

func (md *MeetingDetector) runIdleLoop() {
	ticker := time.NewTicker(config.AppCheckInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		appName, browserURL, _, err := md.spClient.GetCurrentApp()
		if err != nil {
			continue
		}

		isMeetingApp := screenpipe.IsMeetingApp(appName)
		isMeetingURL := screenpipe.IsMeetingURL(browserURL)

		if isMeetingApp || isMeetingURL {
			detectedApp := appName
			if isMeetingURL {
				detectedApp = fmt.Sprintf("%s (%s)", appName, browserURL)
			}
			log.Printf("📱 Meeting app detected: %s\n", detectedApp)

			md.state.Phase = models.PhaseDetecting
			md.state.App = detectedApp
			md.state.StableCount = 0
			md.state.FailureCount = 0

			md.runDetectingPhase()

			if md.state.Phase == models.PhaseIdle {
				log.Println("🔄 Back to IDLE. Watching for meeting apps...\n")
				continue
			}
		}
	}
}

func (md *MeetingDetector) runDetectingPhase() {
	log.Println("🔎 Starting network detection...")

	pid, err := md.network.FindProcessPID(md.state.App)
	if err != nil {
		log.Printf("⚠️  Could not find PID: %v\n", err)
		md.state.Phase = models.PhaseIdle
		return
	}

	md.state.PID = pid
	log.Printf("   PID: %d\n", pid)
	log.Printf("   Checking every 5s for %d sec stability...\n\n", config.StabilityWindowSec)

	ticker := time.NewTicker(config.StabilityCheckInterval)
	defer ticker.Stop()

	timeout := time.After(time.Duration(config.DetectionTimeoutSec) * time.Second)
	checkNum := 0

	for {
		select {
		case <-timeout:
			log.Println("⏰ Detection timeout! No stable connection found.")
			md.state.Phase = models.PhaseIdle
			return

		case <-ticker.C:
			checkNum++
			status := md.network.CheckProcess(pid)

			if !status.ProcessAlive {
				log.Printf("   Check %d: Process DEAD!\n", checkNum)
				md.state.Phase = models.PhaseIdle
				return
			}

			isStable := status.TCPCount > 0 && status.HasUDP

			if isStable {
				md.state.StableCount++
				md.state.ActiveTCPCount = status.TCPCount
				md.state.HasUDP = status.HasUDP
			} else {
				md.state.StableCount = 0
			}

			icon := "❌"
			if isStable {
				icon = "✅"
			}
			log.Printf("   Check %d: TCP=%d UDP=%v Stable=%d/%d %s\n",
				checkNum, status.TCPCount, status.HasUDP,
				md.state.StableCount, config.StabilityThreshold, icon)

			if md.state.StableCount >= config.StabilityThreshold {
				md.onMeetingStarted()
				return
			}
		}
	}
}

func (md *MeetingDetector) onMeetingStarted() {
	md.state.Phase = models.PhaseActive
	md.state.StartTime = time.Now()
	md.state.FailureCount = 0

	logs.LogMeetingStarted(md.state.App, md.state.StartTime)

	// Save baseline TCP count to detect significant drops
	md.state.BaselineTCP = md.state.ActiveTCPCount

	log.Println()
	log.Println("🟢 ══════════════════════════════════════")
	log.Println("║        MEETING STARTED!                ║")
	log.Println("══════════════════════════════════════════")
	log.Printf("   App:         %s\n", md.state.App)
	log.Printf("   PID:         %d\n", md.state.PID)
	log.Printf("   Time:        %s\n", md.state.StartTime.Format("3:04:05 PM"))
	log.Printf("   TCP:         %d connections\n", md.state.ActiveTCPCount)
	log.Printf("   UDP:         %v\n", md.state.HasUDP)
	log.Printf("   Monitoring every 30s...\n\n")

	md.speakerTracker.Reset()
	md.runActivePhase()
}

func (md *MeetingDetector) runActivePhase() {
	ticker := time.NewTicker(config.ActiveCheckInterval)
	defer ticker.Stop()

	checkNum := 0
	consecutiveNetworkDrop := 0

	// Meeting end URLs — yeh dikhein toh PAKKA meeting khatam
	meetingEndURLPatterns := []string{
		"web.webex.com/meetings",
		"meet.google.com/landing",
		"teams.live.com/v2/", // <-- add (Teams lobby after leave)
		"teams.microsoft.com/v2/",
	}

	for {
		<-ticker.C
		checkNum++

		elapsed := time.Since(md.state.StartTime).Round(time.Second)

		// CHECK 1: Screenpipe se URL check — SIRF agar meeting tab pe ho
		_, browserURL, _, err := md.spClient.GetCurrentApp()
		if err == nil && browserURL != "" {
			lowerURL := strings.ToLower(browserURL)

			// Kya ABHI meeting tab pe hai AND meeting end page dikha raha hai?
			isMeetingTab := screenpipe.IsMeetingURL(browserURL)
			if isMeetingTab {
				// User meeting tab pe hai — check karo end page toh nahi
				for _, endPattern := range meetingEndURLPatterns {
					if strings.Contains(lowerURL, endPattern) {
						// Meeting tab pe hai aur lobby/end page dikha raha = PAKKA END
						log.Printf("   [%d] 🔴 Meeting tab shows end page: %s (elapsed: %v)\n",
							checkNum, browserURL, elapsed)
						md.state.EndReason = "meeting_left_url_changed"
						md.onMeetingEnded()
						return
					}
				}
				// Meeting tab pe hai aur meeting URL active — sab theek
				consecutiveNetworkDrop = 0
				log.Printf("   [%d] ✅ On meeting tab, URL active (elapsed: %v)\n",
					checkNum, elapsed)
				md.updateSpeakerData()
				continue
			}
			// User dusre tab pe hai — SIRF network se check karo (below)
		}

		// CHECK 2: Network check (jab dusre tab/window pe ho)
		status := md.network.CheckProcess(md.state.PID)

		if status.HasUDP {
			consecutiveNetworkDrop = 0
			log.Printf("   [%d] ✅ TCP=%d UDP=%v (other tab, network active) (elapsed: %v)\n",
				checkNum, status.TCPCount, status.HasUDP, elapsed)
		} else if !status.ProcessAlive {
			log.Printf("   [%d] 🔴 Browser process DEAD! (elapsed: %v)\n", checkNum, elapsed)
			md.state.EndReason = "browser_closed"
			md.onMeetingEnded()
			return
		} else {
			consecutiveNetworkDrop++
			log.Printf("   [%d] ⚠️  TCP=%d UDP=%v — No media! (%d/%d) (elapsed: %v)\n",
				checkNum, status.TCPCount, status.HasUDP,
				consecutiveNetworkDrop, config.FailureThreshold, elapsed)
		}

		if consecutiveNetworkDrop >= config.FailureThreshold {
			md.state.EndReason = "meeting_ended_no_media"
			md.onMeetingEnded()
			return
		}

		md.updateSpeakerData()
	}
}

func (md *MeetingDetector) updateSpeakerData() {
	startTime := time.Now().Add(-60 * time.Second)
	endTime := time.Now()

	entries, err := md.spClient.GetAudioTranscriptions(startTime, endTime)
	if err != nil {
		return
	}

	for _, entry := range entries {
		md.speakerTracker.AddEntry(entry)
	}
}

func (md *MeetingDetector) onMeetingEnded() {
	md.state.Phase = models.PhaseReporting
	md.state.EndTime = time.Now()
	duration := md.state.EndTime.Sub(md.state.StartTime)

	logs.LogMeetingEnded(md.state.App, md.state.StartTime, md.state.EndTime, md.state.EndReason)

	log.Println()
	log.Println("🔴 ══════════════════════════════════════")
	log.Println("║        MEETING ENDED!                  ║")
	log.Println("══════════════════════════════════════════")
	log.Printf("   App:      %s\n", md.state.App)
	log.Printf("   Reason:   %s\n", md.state.EndReason)
	log.Printf("   Start:    %s\n", md.state.StartTime.Format("3:04:05 PM"))
	log.Printf("   End:      %s\n", md.state.EndTime.Format("3:04:05 PM"))
	log.Printf("   Duration: %v\n", duration.Round(time.Second))
	log.Println()

	log.Println("📊 Fetching transcription data...")
	allEntries, err := md.spClient.GetAudioTranscriptions(
		md.state.StartTime, md.state.EndTime,
	)
	if err != nil {
		log.Printf("⚠️  Error fetching transcriptions: %v\n", err)
	} else {
		for _, entry := range allEntries {
			md.speakerTracker.AddEntry(entry)
		}
	}

	meetingReport := md.reportGen.Generate(
		md.state.App,
		md.state.StartTime,
		md.state.EndTime,
		md.state.EndReason,
		md.speakerTracker.GetSpeakerStats(),
		md.speakerTracker.GetTranscriptions(),
	)

	md.reportGen.PrintReport(meetingReport)

	md.state = models.MeetingState{
		Phase: models.PhaseIdle,
	}
	md.speakerTracker.Reset()

	log.Println("\n🔄 Back to IDLE. Watching for next meeting...\n")
	md.runIdleLoop()
}

// Helper to extract meeting URL from text
func extractMeetingURLFromText(text string) string {
	lower := strings.ToLower(text)
	patterns := []string{"meet.google.com", "teams.microsoft.com", "teams.live.com", "zoom.us"}
	for _, p := range patterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			end := idx
			for end < len(text) && text[end] != ' ' && text[end] != '\n' && text[end] != '\r' {
				end++
			}
			return text[idx:end]
		}
	}
	return ""
}
