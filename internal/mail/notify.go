package mail

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Notify posts a short message to an ntfy topic URL.
//
// Deprecated: operator alerts go through internal/notify, which gives every
// Frazile service the same title, tags, priority and click-through. This
// remains only for anything outside the web handlers that still calls it.
func Notify(ctx context.Context, topicURL, title, body string, tags ...string) {
	if topicURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, topicURL, strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Title", title)
	if len(tags) > 0 {
		req.Header.Set("Tags", strings.Join(tags, ","))
	}
	res, err := http.DefaultClient.Do(req)
	if err == nil {
		res.Body.Close()
	}
}
