package report

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SarangDandekar/Screenpipe/meeting-coach/models"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a MeetingReport from collected data
func (g *Generator) Generate(
	app string,
	startTime, endTime time.Time,
	endReason string,
	speakers []models.SpeakerInfo,
	transcriptions []models.TranscriptionEntry,
) models.MeetingReport {
	duration := endTime.Sub(startTime)

	totalWords := 0
	for _, s := range speakers {
		totalWords += s.WordCount
	}

	return models.MeetingReport{
		App:           app,
		StartTime:     startTime,
		EndTime:       endTime,
		DurationMins:  duration.Minutes(),
		EndReason:     endReason,
		Speakers:      speakers,
		Transcription: transcriptions,
		TotalWords:    totalWords,
	}
}

// PrintReport displays the meeting report in a nice format
func (g *Generator) PrintReport(r models.MeetingReport) {
	log.Println()
	log.Println("╔══════════════════════════════════════════════════╗")
	log.Println("║              📊 MEETING REPORT                  ║")
	log.Println("╠══════════════════════════════════════════════════╣")
	log.Printf("║  App:       %-37s ║\n", r.App)
	log.Printf("║  Start:     %-37s ║\n", r.StartTime.Format("3:04:05 PM"))
	log.Printf("║  End:       %-37s ║\n", r.EndTime.Format("3:04:05 PM"))
	log.Printf("║  Duration:  %-37s ║\n", fmt.Sprintf("%.1f minutes", r.DurationMins))
	log.Printf("║  Reason:    %-37s ║\n", r.EndReason)
	log.Printf("║  Words:     %-37s ║\n", fmt.Sprintf("%d total", r.TotalWords))
	log.Println("╠══════════════════════════════════════════════════╣")
	log.Println("║  🎤 SPEAKER BREAKDOWN                           ║")
	log.Println("╠══════════════════════════════════════════════════╣")

	if len(r.Speakers) == 0 {
		log.Println("║  No speaker data available                       ║")
	}

	for _, s := range r.Speakers {
		mins := s.DurationSecs / 60.0
		bar := g.makeBar(s.Percentage, 20)

		log.Printf("║  %-12s %5.1f min (%4.1f%%) %s ║\n",
			truncate(s.Name, 12),
			mins,
			s.Percentage,
			bar,
		)
		log.Printf("║  %14s %d words, ~%.0f WPM %18s ║\n",
			"",
			s.WordCount,
			func() float64 {
				if mins > 0 {
					return float64(s.WordCount) / mins
				}
				return 0
			}(),
			"",
		)
	}

	log.Println("╠══════════════════════════════════════════════════╣")
	log.Println("║  💡 COACHING INSIGHTS                           ║")
	log.Println("╠══════════════════════════════════════════════════╣")

	insights := g.generateInsights(r)
	for _, insight := range insights {
		log.Printf("║  %s %-45s ║\n", insight.Icon, insight.Text)
	}

	log.Println("╠══════════════════════════════════════════════════╣")
	log.Println("║  📝 TRANSCRIPTION (last 10 entries)             ║")
	log.Println("╠══════════════════════════════════════════════════╣")

	// Last 10 transcriptions
	start := 0
	if len(r.Transcription) > 10 {
		start = len(r.Transcription) - 10
	}
	for _, t := range r.Transcription[start:] {
		speaker := t.SpeakerName
		if speaker == "" {
			speaker = fmt.Sprintf("Speaker %d", t.SpeakerID)
		}
		text := truncate(t.Transcription, 35)
		log.Printf("║  [%-10s] %-35s ║\n", truncate(speaker, 10), text)
	}

	log.Println("╚══════════════════════════════════════════════════╝")

	// Save JSON report
	g.saveJSON(r)
}

type insight struct {
	Icon string
	Text string
}

func (g *Generator) generateInsights(r models.MeetingReport) []insight {
	insights := []insight{}

	if len(r.Speakers) == 0 {
		insights = append(insights, insight{"ℹ️", "No speaker data to analyze"})
		return insights
	}

	// Talk time balance check
	if len(r.Speakers) >= 2 {
		topPct := r.Speakers[0].Percentage
		if topPct > 70 {
			insights = append(insights, insight{"⚠️",
				fmt.Sprintf("%s dominated (%.0f%%). Try more balance.",
					r.Speakers[0].Name, topPct)})
		} else if topPct > 50 {
			insights = append(insights, insight{"🟡",
				fmt.Sprintf("%s spoke most (%.0f%%). Acceptable.",
					r.Speakers[0].Name, topPct)})
		} else {
			insights = append(insights, insight{"✅", "Good talk time balance!"})
		}
	}

	// Meeting duration check
	if r.DurationMins > 60 {
		insights = append(insights, insight{"⏰",
			fmt.Sprintf("Long meeting (%.0f min). Consider shorter.", r.DurationMins)})
	} else if r.DurationMins < 5 {
		insights = append(insights, insight{"⚡", "Very quick meeting. Efficient!"})
	}

	// Words per minute check
	for _, s := range r.Speakers {
		mins := s.DurationSecs / 60.0
		if mins > 1 {
			wpm := float64(s.WordCount) / mins
			if wpm > 180 {
				insights = append(insights, insight{"🏃",
					fmt.Sprintf("%s speaking fast (%.0f WPM).", s.Name, wpm)})
			}
		}
	}

	// Participation check
	if len(r.Speakers) == 1 {
		insights = append(insights, insight{"👤",
			"Only 1 speaker detected. Monologue?"})
	}

	return insights
}

func (g *Generator) makeBar(percentage float64, width int) string {
	filled := int(percentage / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func (g *Generator) saveJSON(r models.MeetingReport) {
	dir := "reports"
	os.MkdirAll(dir, 0755)

	filename := fmt.Sprintf("meeting_%s.json",
		r.StartTime.Format("2006-01-02_15-04-05"))
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Printf("⚠️  Could not save report: %v\n", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("⚠️  Could not write report file: %v\n", err)
		return
	}

	log.Printf("\n💾 Report saved: %s\n", path)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
