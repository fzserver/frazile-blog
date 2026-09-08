// Package mail sends transactional e-mail through Resend's HTTP API.
//
// Only the stdlib is involved: one JSON POST per message, attachments
// base64-inline. The sender address must be on a domain verified in the
// Resend account; nothing here is Frazile-specific beyond that.
package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Attachment is a file to include with the message.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Message is one outgoing e-mail.
type Message struct {
	To          string
	Subject     string
	Text        string
	HTML        string
	ReplyTo     string
	Attachments []Attachment
}

// Sender posts to Resend. A nil *Sender is a valid no-op, so callers can
// keep one field and not branch on whether mail is configured.
type Sender struct {
	key      string
	from     string
	endpoint string
	client   *http.Client
}

// New returns a sender, or nil when either the key or the from address is
// missing (mail off).
func New(apiKey, from string) *Sender {
	if apiKey == "" || from == "" {
		return nil
	}
	return &Sender{key: apiKey, from: from, endpoint: "https://api.resend.com/emails",
		client: &http.Client{Timeout: 20 * time.Second}}
}

// NewWithEndpoint is New against a different API base, for tests.
func NewWithEndpoint(apiKey, from, endpoint string) *Sender {
	s := New(apiKey, from)
	if s != nil {
		s.endpoint = endpoint
	}
	return s
}

// From is the configured sender address.
func (s *Sender) From() string {
	if s == nil {
		return ""
	}
	return s.from
}

// Send delivers one message and returns Resend's message id.
func (s *Sender) Send(ctx context.Context, m Message) (string, error) {
	if s == nil {
		return "", errors.New("mail not configured")
	}
	type att struct {
		Filename    string `json:"filename"`
		Content     string `json:"content"`
		ContentType string `json:"content_type,omitempty"`
	}
	body := map[string]any{
		"from":    s.from,
		"to":      []string{m.To},
		"subject": m.Subject,
	}
	if m.Text != "" {
		body["text"] = m.Text
	}
	if m.HTML != "" {
		body["html"] = m.HTML
	}
	if m.ReplyTo != "" {
		body["reply_to"] = m.ReplyTo
	}
	if len(m.Attachments) > 0 {
		atts := make([]att, 0, len(m.Attachments))
		for _, a := range m.Attachments {
			atts = append(atts, att{a.Filename, base64.StdEncoding.EncodeToString(a.Data), a.ContentType})
		}
		body["attachments"] = atts
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	// Resend sits behind Cloudflare, which rejects Go's default agent string.
	req.Header.Set("User-Agent", "frazile-blog/1.0")
	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("resend: %s: %s", res.Status, bytes.TrimSpace(out))
	}
	var r struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(out, &r)
	return r.ID, nil
}
