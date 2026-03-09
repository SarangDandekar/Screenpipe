package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/SarangDandekar/Screenpipe/meeting-coach/models"
	"github.com/SarangDandekar/Screenpipe/meeting-coach/screenpipe"
)

// ActivityEntry represents a single activity record
type ActivityEntry struct {
	AppName     string    `json:"app_name"`
	WindowTitle string    `json:"window_title"`
	URL         string    `json:"url,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    int64     `json:"duration_seconds"`
	IsMeeting   bool      `json:"is_meeting"`
}

// ActivityTracker tracks all app activities
type ActivityTracker struct {
	entries    []ActivityEntry
	currentApp string
	currentURL string
	startTime  time.Time
	mu         map[string]*time.Time
}

func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{
		mu: make(map[string]*time.Time),
	}
}

// Update updates the current activity with new app/URL info
func (t *ActivityTracker) Update(appName, browserURL string) {
	now := time.Now()

	// Track app usage duration
	if t.currentApp != "" && t.currentApp != appName {
		// Save previous app entry
		duration := now.Sub(t.startTime).Seconds()
		entry := ActivityEntry{
			AppName:     t.currentApp,
			WindowTitle: "",
			URL:         t.currentURL,
			StartTime:   t.startTime,
			EndTime:     now,
			Duration:    int64(duration),
			IsMeeting:   screenpipe.IsMeetingApp(t.currentApp) || screenpipe.IsMeetingURL(t.currentURL),
		}
		t.entries = append(t.entries, entry)
	}

	// Start tracking new app
	t.currentApp = appName
	t.currentURL = browserURL
	t.startTime = now
}

// Flush saves the current activity entry
func (t *ActivityTracker) Flush() {
	if t.currentApp == "" {
		return
	}

	now := time.Now()
	duration := now.Sub(t.startTime).Seconds()
	entry := ActivityEntry{
		AppName:     t.currentApp,
		WindowTitle: "",
		URL:         t.currentURL,
		StartTime:   t.startTime,
		EndTime:     now,
		Duration:    int64(duration),
		IsMeeting:   screenpipe.IsMeetingApp(t.currentApp) || screenpipe.IsMeetingURL(t.currentURL),
	}
	t.entries = append(t.entries, entry)
}

// GetTodayEntries returns all entries for today
func (t *ActivityTracker) GetTodayEntries() []ActivityEntry {
	today := time.Now().Truncate(24 * time.Hour)
	var result []ActivityEntry

	for _, entry := range t.entries {
		if entry.StartTime.After(today) || entry.StartTime.Equal(today) {
			result = append(result, entry)
		}
	}

	// Add current entry if active today
	if t.currentApp != "" && (t.startTime.After(today) || t.startTime.Equal(today)) {
		now := time.Now()
		duration := now.Sub(t.startTime).Seconds()
		result = append(result, ActivityEntry{
			AppName:     t.currentApp,
			WindowTitle: "",
			URL:         t.currentURL,
			StartTime:   t.startTime,
			EndTime:     now,
			Duration:    int64(duration),
			IsMeeting:   screenpipe.IsMeetingApp(t.currentApp) || screenpipe.IsMeetingURL(t.currentURL),
		})
	}

	return result
}

// AppSummary represents summary of app usage
type AppSummary struct {
	AppName       string `json:"app_name"`
	TotalDuration int64  `json:"total_duration_seconds"`
	SessionCount  int    `json:"session_count"`
	IsMeetingApp  bool   `json:"is_meeting_app"`
}

// GetAppSummary returns summary of app usage
func (t *ActivityTracker) GetAppSummary() []AppSummary {
	today := time.Now().Truncate(24 * time.Hour)
	summaryMap := make(map[string]*AppSummary)

	for _, entry := range t.entries {
		if entry.StartTime.After(today) || entry.StartTime.Equal(today) {
			if _, ok := summaryMap[entry.AppName]; !ok {
				summaryMap[entry.AppName] = &AppSummary{
					AppName:       entry.AppName,
					TotalDuration: 0,
					SessionCount:  0,
					IsMeetingApp:  screenpipe.IsMeetingApp(entry.AppName),
				}
			}
			summaryMap[entry.AppName].TotalDuration += entry.Duration
			summaryMap[entry.AppName].SessionCount++
		}
	}

	// Add current session if active today
	if t.currentApp != "" && (t.startTime.After(today) || t.startTime.Equal(today)) {
		if _, ok := summaryMap[t.currentApp]; !ok {
			summaryMap[t.currentApp] = &AppSummary{
				AppName:       t.currentApp,
				TotalDuration: 0,
				SessionCount:  0,
				IsMeetingApp:  screenpipe.IsMeetingApp(t.currentApp),
			}
		}
		summaryMap[t.currentApp].TotalDuration += int64(time.Now().Sub(t.startTime).Seconds())
		summaryMap[t.currentApp].SessionCount++
	}

	var result []AppSummary
	for _, s := range summaryMap {
		result = append(result, *s)
	}

	// Sort by duration descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalDuration > result[j].TotalDuration
	})

	return result
}

// AddMeetingRecord adds a meeting record
func (t *ActivityTracker) AddMeetingRecord(meeting models.MeetingReport) {
	durationSeconds := int64(meeting.DurationMins * 60)
	entry := ActivityEntry{
		AppName:     meeting.App,
		WindowTitle: "",
		URL:         "",
		StartTime:   meeting.StartTime,
		EndTime:     meeting.EndTime,
		Duration:    durationSeconds,
		IsMeeting:   true,
	}
	t.entries = append(t.entries, entry)
}

// GetMeetingSummary returns all meeting records
func (t *ActivityTracker) GetMeetingSummary() []ActivityEntry {
	var meetings []ActivityEntry
	for _, entry := range t.entries {
		if entry.IsMeeting {
			meetings = append(meetings, entry)
		}
	}

	// Sort by start time descending
	sort.Slice(meetings, func(i, j int) bool {
		return meetings[i].StartTime.After(meetings[j].StartTime)
	})

	return meetings
}

// ExportJSON exports activity logs to JSON file
func ExportJSON() {
	tracker := NewActivityTracker()

	entries := tracker.GetTodayEntries()
	summary := tracker.GetAppSummary()

	data := map[string]interface{}{
		"exported_at":   time.Now().Format(time.RFC3339),
		"today_entries": entries,
		"app_summary":   summary,
		"total_apps":    len(summary),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	filename := fmt.Sprintf("logs/activity_%s.json", time.Now().Format("2006-01-02"))
	os.MkdirAll("logs", 0755)
	os.WriteFile(filename, jsonData, 0644)
	fmt.Printf("Exported to %s\n", filename)
}

// FormatDuration formats duration in human readable format
func FormatDuration(seconds int64) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// GetBrowserURLSummary returns summary of browser URLs
func (t *ActivityTracker) GetBrowserURLSummary() map[string]int64 {
	today := time.Now().Truncate(24 * time.Hour)
	urlDurations := make(map[string]int64)

	for _, entry := range t.entries {
		if entry.StartTime.After(today) || entry.StartTime.Equal(today) {
			if entry.URL != "" {
				// Extract domain from URL
				domain := extractDomain(entry.URL)
				if domain != "" {
					urlDurations[domain] += entry.Duration
				}
			}
		}
	}

	return urlDurations
}

func extractDomain(url string) string {
	// Simple domain extraction
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	if idx := strings.Index(url, "/"); idx > 0 {
		url = url[:idx]
	}

	return url
}
