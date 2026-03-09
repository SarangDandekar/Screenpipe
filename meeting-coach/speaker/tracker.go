package speaker

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/your-username/meeting-coach/models"
)

// Tracker accumulates speaker statistics during a meeting
type Tracker struct {
	mu             sync.Mutex
	speakers       map[int]*speakerData
	transcriptions []models.TranscriptionEntry
	seenChunks     map[string]bool // Dedup key: "speakerID-startTime"
}

type speakerData struct {
	ID           int
	Name         string
	TotalSecs    float64
	WordCount    int
	IsInputDevice bool // true = "You" (mic)
}

func NewTracker() *Tracker {
	return &Tracker{
		speakers:   make(map[int]*speakerData),
		seenChunks: make(map[string]bool),
	}
}

// Reset clears all tracking data for a new meeting
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.speakers = make(map[int]*speakerData)
	t.transcriptions = nil
	t.seenChunks = make(map[string]bool)
}

// AddEntry adds a transcription entry (with deduplication)
func (t *Tracker) AddEntry(entry models.TranscriptionEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Dedup: same speaker + same start time = same chunk
	dedupKey := fmt.Sprintf("%d-%.3f", entry.SpeakerID, entry.StartTime)
	if t.seenChunks[dedupKey] {
		return
	}
	t.seenChunks[dedupKey] = true

	// Duration calculate karo
	duration := entry.EndTime - entry.StartTime
	if duration <= 0 {
		duration = 0
	}

	// Word count
	words := len(strings.Fields(entry.Transcription))

	// Speaker data update karo
	sd, exists := t.speakers[entry.SpeakerID]
	if !exists {
		name := entry.SpeakerName
		if name == "" {
			if entry.IsInput {
				name = "You"
			} else {
				name = fmt.Sprintf("Speaker %d", entry.SpeakerID)
			}
		}

		sd = &speakerData{
			ID:            entry.SpeakerID,
			Name:          name,
			IsInputDevice: entry.IsInput,
		}
		t.speakers[entry.SpeakerID] = sd
	}

	sd.TotalSecs += duration
	sd.WordCount += words

	// Transcription store karo
	t.transcriptions = append(t.transcriptions, entry)
}

// GetSpeakerStats returns sorted speaker statistics
func (t *Tracker) GetSpeakerStats() []models.SpeakerInfo {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Total speaking time calculate karo
	totalSecs := 0.0
	for _, sd := range t.speakers {
		totalSecs += sd.TotalSecs
	}

	// SpeakerInfo list banao
	stats := make([]models.SpeakerInfo, 0, len(t.speakers))
	for _, sd := range t.speakers {
		pct := 0.0
		if totalSecs > 0 {
			pct = (sd.TotalSecs / totalSecs) * 100
		}

		stats = append(stats, models.SpeakerInfo{
			ID:           sd.ID,
			Name:         sd.Name,
			DurationSecs: sd.TotalSecs,
			Percentage:   pct,
			WordCount:    sd.WordCount,
		})
	}

	// Sort by duration (highest first)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].DurationSecs > stats[j].DurationSecs
	})

	return stats
}

// GetTranscriptions returns all stored transcriptions in order
func (t *Tracker) GetTranscriptions() []models.TranscriptionEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Sort by start time
	sorted := make([]models.TranscriptionEntry, len(t.transcriptions))
	copy(sorted, t.transcriptions)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime < sorted[j].StartTime
	})

	return sorted
}