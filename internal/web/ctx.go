package web

import (
	"context"

	"github.com/fzserver/frazile-blog/internal/mail"
)

func mailMessage(to, subject, text, replyTo string) mail.Message {
	return mail.Message{To: to, Subject: subject, Text: text, ReplyTo: replyTo}
}

// contactMail forwards what someone wrote in the contact form, in the same
// shape as every other Frazile e-mail, with their address as the reply-to
// so answering it just works.
func contactMail(to, subject, body, from, fromEmail, ip string) mail.Message {
	return mail.Page{
		Product: "Frazile Blog",
		Title:   "New message from the contact form",
		Rows: []mail.Row{
			{Label: "From", Value: from},
			{Label: "E-mail", Value: fromEmail},
			{Label: "Subject", Value: subject},
			{Label: "Sent from", Value: ip},
		},
		Body:    body,
		Note:    "Reply to this e-mail and it goes straight back to them.",
		Site:    "blog.frazile.com",
		Contact: "hello@frazile.com",
	}.Message(to, "[Contact] "+subject, fromEmail)
}

func contextBG() context.Context { return context.Background() }

func notifyFn(url, title, body string, tags ...string) {
	mail.Notify(context.Background(), url, title, body, tags...)
}
