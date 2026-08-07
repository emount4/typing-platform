package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	UserID          int64   `json:"user_id"`
	TextID          string  `json:"text_id"`
	WPM             float64 `json:"wpm"`
	Accuracy        float64 `json:"accuracy"`
	DurationMs      int64   `json:"duration_ms"`
	TotalKeystrokes int     `json:"total_keystrokes"`
	Errors          int     `json:"errors"`
	IsPersonalBest  bool    `json:"is_personal_best"`
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

func (c *ApiClient) SubmitRun(ctx context.Context, run *RunResult) error {
	url := fmt.Sprintf("%s/internal/runs", c.baseURL)

	body, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}

	bodyReader := bytes.NewReader(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}
