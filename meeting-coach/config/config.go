package config

import "time"

// Detection phase: Jab app detect ho, network check karo
const (
	// Har kitne seconds pe screenpipe se app check karna hai (IDLE phase)
	AppCheckInterval = 5 * time.Second

	// Network stability check: har kitne sec pe (DETECTING phase)
	StabilityCheckInterval = 3 * time.Second

	// Kitne sec stable rehna chahiye = meeting confirmed
	// 15 sec / 5 sec interval = 3 consecutive checks
	StabilityWindowSec = 9
	StabilityThreshold = StabilityWindowSec / 5 // = 3 checks

	// Agar 60 sec mein stable nahi hua toh abort
	DetectionTimeoutSec = 60
)

// Active monitoring phase: Meeting chalu hai
const (
	// Har kitne sec pe network check (ACTIVE phase)
	ActiveCheckInterval = 15 * time.Second

	// Kitne consecutive failures pe meeting end
	FailureThreshold = 2
)

// Meeting apps jo hum detect karte hain
var MeetingApps = map[string]bool{
	"zoom":            true,
	"zoom.us":         true,
	"microsoft teams": true,
	"teams":           true,
	"facetime":        true,
	"webex":           true,
	"skype":           true,
	"slack":           true,
	"google meet":     true,
	"discord":         true,
}

// Browser URL patterns for web-based meetings
var BrowserMeetingURLs = []string{
	"meet.google.com",
	"teams.microsoft.com",
	"zoom.us/j",
	"zoom.us/wc",
	"whereby.com",
	"app.slack.com/huddle",
	"webex.com",            // <-- add
	"web.webex.com",        // <-- add
	"meet.lync.com",        // <-- add (Skype for Business)
	"discord.com/channels",
	 "teams.live.com",    // <-- add (Discord voice)
}

// Screenpipe API
const ScreenpipeBaseURL = "http://localhost:3030"
