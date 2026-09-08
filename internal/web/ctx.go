package web

import (
	"context"

	"github.com/fzserver/frazile-blog/internal/mail"
)

func contextBG() context.Context { return context.Background() }

func notifyFn(url, title, body string, tags ...string) {
	mail.Notify(context.Background(), url, title, body, tags...)
}
