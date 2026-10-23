package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrDisabled = errors.New("claude API key tidak diset")

type Client struct {
	apiKey     string
	model      string
	apiURL     string
	httpClient *http.Client
}

func NewClient(apiKey, model, apiURL string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
	System    string    `json:"system"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type response struct {
	Content []contentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete mengirim prompt ke Claude dan mengembalikan gabungan blok teks jawaban.
func (c *Client) Complete(system, userPrompt string) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}

	body, err := json.Marshal(request{
		Model:     c.model,
		MaxTokens: 1500,
		System:    system,
		Messages:  []message{{Role: "user", Content: userPrompt}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result response
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("claude API status %d: respons bukan JSON", resp.StatusCode)
	}
	if result.Error != nil {
		return "", fmt.Errorf("claude API status %d: %s", resp.StatusCode, result.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API status %d", resp.StatusCode)
	}

	var text strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if text.Len() == 0 {
		return "", errors.New("claude API: respons kosong")
	}
	return text.String(), nil
}
