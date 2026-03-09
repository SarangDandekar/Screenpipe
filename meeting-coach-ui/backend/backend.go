package backend

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/SarangDandekar/Screenpipe/meeting-coach/detector"
	"github.com/SarangDandekar/Screenpipe/meeting-coach/logs"
	"github.com/SarangDandekar/Screenpipe/meeting-coach/screenpipe"
)

// EventWriter writes logs to stdout and emits them to frontend
type EventWriter struct {
	mu  sync.Mutex
	ctx context.Context
}

func (w *EventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Also print on stdout for CLI visibility
	_, _ = os.Stdout.Write(p)

	if w.ctx != nil {
		line := strings.TrimRight(string(p), "\r\n")
		_ = runtime.EventsEmit(w.ctx, "detector:log", map[string]interface{}{"line": line})
	}
	return len(p), nil
}

// App is the Wails backend exposed to frontend
type App struct {
	ctx      context.Context
	md       *detector.MeetingDetector
	spClient *screenpipe.Client
	tracker  *logs.ActivityTracker
	writer   *EventWriter
	ticker   *time.Ticker
}

func NewApp() *App {
	return &App{
		md:       detector.NewMeetingDetector(),
		spClient: screenpipe.NewClient(),
		tracker:  logs.NewActivityTracker(),
		writer:   &EventWriter{},
	}
}

// Start sets up logging redirection, starts detector and polling; called from frontend
func (a *App) Start(ctx context.Context) error {
	a.ctx = ctx

	// Redirect std logger to our EventWriter
	a.writer.mu.Lock()
	a.writer.ctx = ctx
	a.writer.mu.Unlock()
	log.SetFlags(0)
	log.SetOutput(a.writer)

	// Start detector (uses existing detector logic unchanged)
	go func() {
		log.Println("Backend: starting detector...")
		a.md.Start()
	}()

	// Poll Screenpipe every 5s and emit activity snapshots
	a.ticker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				a.ticker.Stop()
				return
			case <-a.ticker.C:
				appName, browserURL, _, err := a.spClient.GetCurrentApp()
				if err != nil {
					_ = runtime.EventsEmit(ctx, "detector:log", map[string]interface{}{
						"line": "Screenpipe unreachable: " + err.Error(),
					})
					continue
				}

				if screenpipe.IsMeetingApp(appName) || screenpipe.IsMeetingURL(browserURL) {
					a.tracker.Flush()
				} else {
					a.tracker.Update(appName, browserURL)
				}

				entries := a.tracker.GetTodayEntries()
				summary := a.tracker.GetAppSummary()
				_ = runtime.EventsEmit(ctx, "activity:update", map[string]interface{}{
					"entries": entries,
					"summary": summary,
				})
			}
		}
	}()

	return nil
}

// ExportJSON triggers logs export and returns exported file path
func (a *App) ExportJSON(ctx context.Context) (string, error) {
	logs.ExportJSON()
	return "logs/activity_export.json", nil
}
