// Package slack posts failure notifications to a Slack incoming webhook.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const maxSectionRunes = 2800 // Slack limits a section to 3000 characters

// Client posts messages to one webhook URL.
type Client struct {
	webhookURL string
	http       *http.Client
}

func New(webhookURL string) *Client {
	return &Client{
		webhookURL: webhookURL,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Notification is everything a Slack message is built from.
type Notification struct {
	Pipeline      string
	Build         string
	BuildURL      string // optional, must already be validated
	Explanation   string
	Fix           string
	AIUnavailable bool   // true when the LLM step failed
	LogTail       string // shown only when AIUnavailable
}

// Slack requires these three characters to be escaped in message text.
var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escape(s string) string { return escaper.Replace(s) }

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

func section(text string) map[string]any {
	return map[string]any{
		"type": "section",
		"text": map[string]any{"type": "mrkdwn", "text": truncate(text, maxSectionRunes)},
	}
}

func buildPayload(n Notification) map[string]any {
	title := "Build failed: " + n.Pipeline
	if n.Build != "" {
		title += " #" + n.Build
	}
	title = truncate(title, 150) // Slack header limit

	meta := fmt.Sprintf("*Pipeline:* %s\n*Build:* #%s", escape(n.Pipeline), escape(n.Build))
	if n.BuildURL != "" {
		meta += fmt.Sprintf("\n<%s|View build in Jenkins>", strings.ReplaceAll(n.BuildURL, "&", "&amp;"))
	}

	blocks := []map[string]any{
		{"type": "header", "text": map[string]any{"type": "plain_text", "text": title}},
		section(meta),
	}

	if n.AIUnavailable {
		blocks = append(blocks, section("_AI analysis was unavailable for this failure._"))
		if tail := strings.TrimSpace(n.LogTail); tail != "" {
			tail = strings.ReplaceAll(escape(tail), "```", "'''")
			blocks = append(blocks, section("*Log tail*\n```"+tail+"```"))
		}
	} else {
		blocks = append(blocks, section("*What went wrong*\n"+escape(n.Explanation)))
		if fix := strings.TrimSpace(n.Fix); fix != "" {
			blocks = append(blocks, section("*Suggested fix*\n"+escape(fix)))
		}
	}

	return map[string]any{
		"text":   title, // fallback shown in push notifications
		"blocks": blocks,
	}
}

// Notify posts the notification to Slack.
func (c *Client) Notify(ctx context.Context, n Notification) error {
	body, err := json.Marshal(buildPayload(n))
	if err != nil {
		return fmt.Errorf("encode slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return errors.New("build slack request failed")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// *url.Error embeds the full URL, and the webhook URL is a secret.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
