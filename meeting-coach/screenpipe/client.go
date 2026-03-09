package screenpipe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/your-username/meeting-coach/config"
	"github.com/your-username/meeting-coach/models"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL: config.ScreenpipeBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type SearchResponse struct {
	Data       []SearchItem `json:"data"`
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"pagination"`
}

type SearchItem struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type OCRContent struct {
	AppName    string  `json:"app_name"`
	WindowName string  `json:"window_name"`
	BrowserURL *string `json:"browser_url"`
	Text       string  `json:"text"`
	Timestamp  string  `json:"timestamp"`
}

type AudioContent struct {
	Transcription string  `json:"transcription"`
	DeviceName    string  `json:"device_name"`
	IsInputDevice bool    `json:"is_input_device"`
	SpeakerID     int     `json:"speaker_id"`
	SpeakerName   string  `json:"speaker_name"`
	StartTime     float64 `json:"start_time"`
	EndTime       float64 `json:"end_time"`
	Duration      float64 `json:"duration"`
}

func (c *Client) GetCurrentApp() (appName string, browserURL string, ocrText string, err error) {
	resp, err := c.httpClient.Get(
		fmt.Sprintf("%s/search?content_type=ocr&limit=1&offset=0", c.baseURL),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("screenpipe unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	var result SearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", "", err
	}

	if len(result.Data) == 0 {
		return "", "", "", nil
	}

	var ocr OCRContent
	if err := json.Unmarshal(result.Data[0].Content, &ocr); err != nil {
		return "", "", "", err
	}

	bURL := ""
	if ocr.BrowserURL != nil {
		bURL = *ocr.BrowserURL
	}

	return ocr.AppName, bURL, ocr.Text, nil
}

func (c *Client) GetAudioTranscriptions(startTime, endTime time.Time) ([]models.TranscriptionEntry, error) {
	params := url.Values{
		"content_type": {"audio"},
		"start_time":   {startTime.UTC().Format(time.RFC3339)},
		"end_time":     {endTime.UTC().Format(time.RFC3339)},
		"limit":        {"1000"},
	}

	allEntries := []models.TranscriptionEntry{}
	offset := 0

	for {
		params.Set("offset", fmt.Sprintf("%d", offset))
		reqURL := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

		resp, err := c.httpClient.Get(reqURL)
		if err != nil {
			return allEntries, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return allEntries, err
		}

		var result SearchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return allEntries, err
		}

		if len(result.Data) == 0 {
			break
		}

		for _, item := range result.Data {
			var audio AudioContent
			if err := json.Unmarshal(item.Content, &audio); err != nil {
				continue
			}

			entry := models.TranscriptionEntry{
				Transcription: audio.Transcription,
				SpeakerID:     audio.SpeakerID,
				SpeakerName:   audio.SpeakerName,
				StartTime:     audio.StartTime,
				EndTime:       audio.EndTime,
				IsInput:       audio.IsInputDevice,
			}
			allEntries = append(allEntries, entry)
		}

		offset += len(result.Data)
		if offset >= result.Pagination.Total {
			break
		}
	}

	return allEntries, nil
}

func IsMeetingApp(appName string) bool {
	lower := strings.ToLower(appName)
	for app := range config.MeetingApps {
		if strings.Contains(lower, app) {
			return true
		}
	}
	return false
}

func IsMeetingURL(browserURL string) bool {
	if browserURL == "" {
		return false
	}
	lower := strings.ToLower(browserURL)
	for _, pattern := range config.BrowserMeetingURLs {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
