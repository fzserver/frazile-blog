// Package notify posts operator alerts to ntfy, in one shape for every
// Frazile service.
//
// The house style, so a phone full of these stays readable:
//
//	Title     Frazile Drive: New account          product, then the event
//	Body      one fact a line, shortest first
//	Tags      one emoji, the same one for the same kind of event everywhere
//	Priority  quiet for routine, high for something that wants a person
//	Click     the page that answers the question the alert raises
//
// Sending is fire-and-forget: an alert that cannot be delivered must never
// hold up, or fail, whatever it was reporting on.
package notify

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Tags, shared by every service so an icon means one thing on the phone.
const (
	TagAccount = "bust_in_silhouette" // somebody signed up
	TagMoney   = "moneybag"           // a payment landed
	TagProblem = "rotating_light"     // something needs a person
	TagWarning = "warning"            // worth knowing, not urgent
	TagAdmin   = "closed_lock_with_key"
	TagComment = "speech_balloon"
	TagMail    = "envelope"
	TagBuild   = "package"
)

// Priorities as ntfy numbers them.
const (
	Quiet  = 2
	Normal = 3
	High   = 4
	Urgent = 5
)

type Event struct {
	Product  string // "Frazile Drive"
	Title    string // "New account"
	Lines    []string
	Tag      string
	Priority int    // 0 leaves ntfy's default
	Click    string // where to go to act on it
}

// Notifier holds the topic to post to. A zero Notifier is valid and does
// nothing, so a service with no topic configured needs no branches.
type Notifier struct {
	URL  string
	Log  *slog.Logger
	HTTP *http.Client
}

func New(topicURL string, log *slog.Logger) *Notifier {
	return &Notifier{URL: topicURL, Log: log, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Send posts one event. It returns at once; the request runs on its own.
func (n *Notifier) Send(e Event) {
	if n == nil || n.URL == "" {
		return
	}
	go n.post(e)
}

// SendContext is Send for a caller that wants to wait, which is only the
// tests and a shutdown path.
func (n *Notifier) SendContext(ctx context.Context, e Event) {
	if n == nil || n.URL == "" {
		return
	}
	n.postContext(ctx, e)
}

func (n *Notifier) post(e Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n.postContext(ctx, e)
}

func (n *Notifier) postContext(ctx context.Context, e Event) {
	title := e.Title
	if e.Product != "" {
		// ASCII only: an HTTP header is Latin-1 by the specification, and a
		// typographic dash here arrives as mojibake on some phones. The
		// body is UTF-8 and can say whatever it likes.
		title = e.Product + ": " + e.Title
	}
	body := strings.Join(e.Lines, "\n")
	if body == "" {
		body = e.Title
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Title", title)
	if e.Tag != "" {
		req.Header.Set("Tags", e.Tag)
	}
	if e.Priority > 0 {
		req.Header.Set("Priority", strconv.Itoa(e.Priority))
	}
	if e.Click != "" {
		req.Header.Set("Click", e.Click)
	}
	client := n.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		if n.Log != nil {
			n.Log.Warn("ntfy unreachable", "err", err)
		}
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && n.Log != nil {
		n.Log.Warn("ntfy refused an alert", "status", res.StatusCode, "title", title)
	}
}
