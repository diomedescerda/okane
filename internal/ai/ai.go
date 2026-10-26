// Package ai provides a minimal, generic client for OpenAI-compatible chat
// completion APIs. It is intentionally transport-level: callers build their own
// prompts and parse their own structured outputs, so the same client powers
// transaction categorization today and future insight features.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	retry   time.Duration
}

// Config tunes the client. BaseURL must point to the versioned API root, for
// example "https://opencode.ai/inference/openai/v1".
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	// Timeout is the per-request timeout; zero defaults to 30s.
	Timeout time.Duration
}

const defaultTimeout = 30 * time.Second

// New returns a client; an empty Model defaults to "glm-5.1".
func New(cfg Config) *Client {
	if cfg.Model == "" {
		cfg.Model = "glm-5.1"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		http:    &http.Client{Timeout: timeout},
		retry:   750 * time.Millisecond,
	}
}

type chatRequest struct {
	Model          string         `json:"model"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat responseFormat `json:"response_format"`
	Messages       []chatMessage  `json:"messages"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one chat completion request and returns the first choice's
// message content. The temperature is 0 and response_format is json_object so
// callers can parse the reply as JSON.
//
// Transient failures (5xx, 429, transport errors) are retried once.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:          c.model,
		Temperature:    0,
		ResponseFormat: responseFormat{Type: "json_object"},
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode chat request: %w", err)
	}

	var content string
	for attempt := 0; ; attempt++ {
		content, err = c.once(ctx, body)
		if err == nil {
			return content, nil
		}
		if !retryable(err) || attempt >= 1 {
			return "", err
		}
		select {
		case <-time.After(c.retry):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (c *Client) once(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", transportError{err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22)) // 4 MiB cap
	if err != nil {
		return "", transportError{fmt.Errorf("read chat response: %w", err)}
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		msg := strings.TrimSpace(string(raw))
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Error.Message != "" {
			msg = apiErr.Error.Message
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", retryableStatus{code: resp.StatusCode, message: msg}
		}
		return "", fmt.Errorf("chat completion: status %d: %s", resp.StatusCode, truncate(msg, 500))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("chat completion: response has no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// SetRetryDelay overrides the backoff between attempts. It exists for tests.
func (c *Client) SetRetryDelay(d time.Duration) { c.retry = d }

// retryable marks errors that are safe to retry once.
type retryableError interface{ retryable() bool }

type transportError struct{ err error }

func (e transportError) Error() string { return e.err.Error() }
func (e transportError) Unwrap() error { return e.err }
func (transportError) retryable() bool { return true }

type retryableStatus struct {
	code    int
	message string
}

func (e retryableStatus) Error() string {
	return fmt.Sprintf("chat completion: status %d: %s", e.code, truncate(e.message, 500))
}
func (retryableStatus) retryable() bool { return true }

func retryable(err error) bool {
	_, ok := err.(retryableError)
	return ok
}

// Assignment is one model-suggested categorization: which transaction gets
// which category. Slug repeats the category slug as a redundancy check; it is
// not authoritative.
type Assignment struct {
	TransactionID string `json:"transaction_id"`
	CategoryID    string `json:"category_id"`
	Slug          string `json:"slug"`
}

// Categorize asks the model to assign category ids to the transactions
// described in userPrompt and parses the structured reply. The prompt must
// describe the candidate categories and the transactions; see the finance
// package for the canonical wording.
func (c *Client) Categorize(ctx context.Context, userPrompt string) ([]Assignment, error) {
	content, err := c.Complete(ctx, categorizeSystem, userPrompt)
	if err != nil {
		return nil, err
	}
	return parseAssignments(content)
}

const categorizeSystem = `You categorize bank transactions for a personal finance app. For each transaction choose exactly one category from the provided list, preferring the transaction description and merchant. Amounts are Colombian pesos (COP): amount below zero or direction "out" is an expense, otherwise it is income, so expense categories fit outflows and income categories fit inflows. Respond with JSON only, shaped like {"assignments":[{"transaction_id":"<transaction id>","category_id":"<category id>","slug":"<category slug>"}]}. If no category fits a transaction, omit it from the response.`

func parseAssignments(content string) ([]Assignment, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var payload struct {
		Assignments []Assignment `json:"assignments"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, fmt.Errorf("parse assignments: %w", err)
	}
	out := make([]Assignment, 0, len(payload.Assignments))
	for _, a := range payload.Assignments {
		if strings.TrimSpace(a.TransactionID) == "" || strings.TrimSpace(a.CategoryID) == "" {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
