package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ApiClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type TextResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Lang    string `json:"lang"`
}

type RunResult struct {
	SessionID  string         `json:"session_id"`
	UserID     *int64         `json:"user_id"`
	AnonID     *string        `json:"anon_id"`
	TextID     string         `json:"text_id"`
	Mode       string         `json:"mode"`
	StartedAt  time.Time      `json:"started_at"`
	DurationMs int64          `json:"duration_ms"`
	WPMNet     float64        `json:"wpm_net"`
	WPMRaw     float64        `json:"wpm_raw"`
	Accuracy   float64        `json:"accuracy"`
	Typed      int            `json:"typed"`
	Correct    int            `json:"correct"`
	Errors     int            `json:"errors"`
	Flags      []string       `json:"flags"`
	Keystrokes []RunKeystroke `json:"keystrokes"`
}

type RunKeystroke struct {
	Char    string `json:"char"`
	T       int64  `json:"t"`
	IsError bool   `json:"is_error"`
}

type RunSubmitResult struct {
	RunID          string   `json:"run_id"`
	IsPersonalBest bool     `json:"is_personal_best"`
	Flagged        bool     `json:"flagged"`
	RatingDelta    *float64 `json:"rating_delta"`
}

func NewApiClient(baseURL, token string) *ApiClient {
	return &ApiClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		token: token,
	}
}

func (c *ApiClient) GetText(ctx context.Context, textID string) (*TextResponse, error) {
	url := fmt.Sprintf("%s/internal/texts/%s", c.baseURL, textID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for python api: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot get text from python api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python api returned status %d", resp.StatusCode)
	}

	var text TextResponse
	if err := json.NewDecoder(resp.Body).Decode(&text); err != nil {
		return nil, fmt.Errorf("cannot decode response from python api: %w", err)
	}

	return &text, nil
}

func (c *ApiClient) SubmitRun(ctx context.Context, run *RunResult) (*RunSubmitResult, error) {
	url := fmt.Sprintf("%s/internal/runs", c.baseURL)

	bodyBytes, err := json.Marshal(run)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal run: %w", err)
	}

	bodyReader := bytes.NewReader(bodyBytes)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	result := &RunSubmitResult{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read run submission response: %w", err)
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return nil, fmt.Errorf("failed to decode run submission response: %w", err)
		}
	}

	return result, nil
}
