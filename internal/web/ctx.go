package web

import (
	"context"

	"github.com/fzserver/frazile-blog/internal/mail"
)

func mailMessage(to, subject, text, replyTo string) mail.Message {
	return mail.Message{To: to, Subject: subject, Text: text, ReplyTo: replyTo}
}

func contextBG() context.Context { return context.Background() }

func notifyFn(url, title, body string, tags ...string) {
	mail.Notify(context.Background(), url, title, body, tags...)
}
