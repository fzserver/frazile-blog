package mail

import (
	"fmt"
)

// Code renders the verification e-mail for one purpose. Plain text and a
// minimal HTML twin; the code is the only thing that matters.
func Code(site, purpose, code string, ttlMin int) (subject, text, htmlBody string) {
	p := Page{
		Product: site,
		Title:   "Your verification code",
		Intro:   "Enter this code to verify your e-mail address and finish signing up.",
		Code:    code,
		Site:    "blog.frazile.com",
		Contact: "hello@frazile.com",
	}
	subject = fmt.Sprintf("%s: your verification code", site)
	switch purpose {
	case "reset":
		subject = fmt.Sprintf("%s: your password reset code", site)
		p.Title, p.Intro = "Your password reset code", "Enter this code to set a new password."
	case "login":
		subject = fmt.Sprintf("%s: your log-in code", site)
		p.Title, p.Intro = "Your log-in code", "Enter this code to finish logging in."
		p.Note = fmt.Sprintf("It expires in %d minutes. If you did not just log in, someone else has your password: they cannot get in without this code, but choose a new password at blog.frazile.com/forgot.", ttlMin)
		return subject, p.Text(), p.HTML()
	case "email":
		subject = fmt.Sprintf("%s: confirm your new e-mail address", site)
		p.Title, p.Intro = "Confirm your new e-mail address", "Enter this code to confirm the address you just gave us."
	}
	p.Note = fmt.Sprintf("It expires in %d minutes. If you did not ask for it, ignore this e-mail: nothing happens without the code.", ttlMin)
	return subject, p.Text(), p.HTML()
}
