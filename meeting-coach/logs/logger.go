package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const logDir = "logs"
const logFile = "activity.log"

func logPath() string {
	return filepath.Join(logDir, logFile)
}

func ensureDir() {
	os.MkdirAll(logDir, 0755)
}

// LogMeetingStarted writes a MEETING_STARTED entry to the activity log
func LogMeetingStarted(app string, startTime time.Time) {
	ensureDir()
	line := fmt.Sprintf("[%s] MEETING_STARTED | App: %s | Start: %s\n",
		startTime.Format("2006-01-02 15:04:05"),
		app,
		startTime.Format("3:04:05 PM"),
	)
	appendLine(line)
}

// LogMeetingEnded writes a MEETING_ENDED entry with duration to the activity log
func LogMeetingEnded(app string, startTime, endTime time.Time, endReason string) {
	ensureDir()
	duration := endTime.Sub(startTime).Round(time.Second)
	durationStr := formatDuration(duration)
	line := fmt.Sprintf("[%s] MEETING_ENDED   | App: %s | Start: %s | End: %s | Duration: %s | Reason: %s\n",
		endTime.Format("2006-01-02 15:04:05"),
		app,
		startTime.Format("3:04:05 PM"),
		endTime.Format("3:04:05 PM"),
		durationStr,
		endReason,
	)
	appendLine(line)
}

// PrintAllLogs prints all activity log entries to stdout in a formatted table
func PrintAllLogs() {
	path := logPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("  No activity logs found yet.")
		fmt.Printf("  (Log file will be created at: %s)\n", path)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		fmt.Println("  No activity logs found yet.")
		return
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║                         📋 ACTIVITY LOG — ALL MEETINGS                     ║")
	fmt.Println("  ╠══════════════════════════════════════════════════════════════════════════════╣")

	meetingCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Printf("  ║  %-76s ║\n", truncate(line, 76))
		if strings.Contains(line, "MEETING_ENDED") {
			meetingCount++
			fmt.Println("  ╠══════════════════════════════════════════════════════════════════════════════╣")
		}
	}

	fmt.Println("  ╠══════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  ║  Total completed meetings: %-49d ║\n", meetingCount)
	fmt.Println("  ╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func appendLine(line string) {
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s + strings.Repeat(" ", max-len(s))
	}
	return s[:max-3] + "..."
}
