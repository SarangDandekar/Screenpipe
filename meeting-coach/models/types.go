package models

import "time"

// MeetingPhase represents the current state of detection
type MeetingPhase int

const (
	PhaseIdle      MeetingPhase = iota // Kuch nahi ho raha
	PhaseDetecting                     // App mila, network check ho raha hai
	PhaseActive                        // Meeting confirmed, monitoring
	PhaseEnding                        // Failures aa rahe hain
	PhaseReporting                     // Meeting khatam, report bana rahe hain
)

func (p MeetingPhase) String() string {
	switch p {
	case PhaseIdle:
		return "IDLE"
	case PhaseDetecting:
		return "DETECTING"
	case PhaseActive:
		return "ACTIVE"
	case PhaseEnding:
		return "ENDING"
	case PhaseReporting:
		return "REPORTING"
	default:
		return "UNKNOWN"
	}
}

// MeetingState holds the current meeting detection state
type MeetingState struct {
	Phase               MeetingPhase
	App                 string
	PID                 int
	StartTime           time.Time
	EndTime             time.Time
	StableCount         int
	FailureCount        int
	ActiveTCPCount      int
	HasUDP              bool
	EndReason           string
	BaselineTCP         int 
}

// NetworkStatus result of a single network check
type NetworkStatus struct {
	ProcessAlive  bool
	TCPCount      int
	HasUDP        bool
	Timestamp     time.Time
	UDPCount     int
}

// SpeakerInfo holds per-speaker statistics
type SpeakerInfo struct {
	ID           int     `json:"speaker_id"`
	Name         string  `json:"name"`
	DurationSecs float64 `json:"duration_secs"`
	Percentage   float64 `json:"percentage"`
	WordCount    int     `json:"word_count"`
}

// TranscriptionEntry represents a single transcription chunk from screenpipe
type TranscriptionEntry struct {
	Transcription string  `json:"transcription"`
	SpeakerID     int     `json:"speaker_id"`
	SpeakerName   string  `json:"speaker_name"`
	StartTime     float64 `json:"start_time"`
	EndTime       float64 `json:"end_time"`
	IsInput       bool    `json:"is_input_device"`
}

// MeetingReport final report after meeting ends
type MeetingReport struct {
	App           string        `json:"app"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	DurationMins  float64       `json:"duration_mins"`
	EndReason     string        `json:"end_reason"`
	Speakers      []SpeakerInfo `json:"speakers"`
	Transcription []TranscriptionEntry `json:"transcription"`
	TotalWords    int           `json:"total_words"`
}