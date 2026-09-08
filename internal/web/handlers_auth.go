package web

import (
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fzserver/frazile-blog/internal/store"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,24}$`)

func validEmail(e string) bool {
	e = strings.TrimSpace(e)
	if len(e) > 254 || !strings.Contains(e, "@") {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e
}

func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if s.user(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	d := s.data(r, "Log in")
	d["Next"] = safeNext(r.URL.Query().Get("next"))
	d["NoIndex"] = true
	s.render(w, r, "login", d)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	login := strings.TrimSpace(r.FormValue("login"))
	pass := r.FormValue("password")
	d := s.data(r, "Log in")
	d["Next"], d["Login"], d["NoIndex"] = next, login, true
	if !s.limit("login:"+s.ip(r), 15, 10*time.Minute) {
		d["Error"] = "Too many attempts. Please wait a few minutes."
		s.renderStatus(w, r, http.StatusTooManyRequests, "login", d)
		return
	}
	u, err := s.db.CheckPassword(r.Context(), login, pass)
	if err != nil {
		if !errors.Is(err, store.ErrBadCredentials) {
			s.fail(w, r, err)
			return
		}
		d["Error"] = "Wrong username or password."
		s.renderStatus(w, r, http.StatusUnauthorized, "login", d)
		return
	}
	if u.Banned {
		d["Error"] = "This account has been suspended."
		s.renderStatus(w, r, http.StatusForbidden, "login", d)
		return
	}
	if !u.Verified {
		// Finish the registration instead: send a fresh code and go verify.
		if err := s.sendCode(r.Context(), u.Email, store.PurposeVerify); err != nil && !errors.Is(err, store.ErrCodeThrottled) {
			s.log.Warn("verify code", "err", err)
		}
		http.Redirect(w, r, "/verify?email="+url.QueryEscape(u.Email), http.StatusSeeOther)
		return
	}
	token, err := s.db.CreateSession(r.Context(), u.ID, r.UserAgent(), s.ip(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.db.DeleteSession(r.Context(), c.Value)
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) {
	s.db.DeleteUserSessions(r.Context(), s.user(r).ID)
	s.clearSessionCookie(w)
	s.flash(w, "ok", "Signed out everywhere.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) registerForm(w http.ResponseWriter, r *http.Request) {
	if s.user(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	d := s.data(r, "Create an account")
	d["NoIndex"] = true
	d["Open"] = s.Settings().RegistrationOpen && s.mailer != nil
	s.render(w, r, "register", d)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Create an account")
	d["NoIndex"] = true
	d["Open"] = s.Settings().RegistrationOpen && s.mailer != nil
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	pass := r.FormValue("password")
	d["Username"], d["Email"] = username, email
	fail := func(msg string, code int) {
		d["Error"] = msg
		s.renderStatus(w, r, code, "register", d)
	}
	if !d["Open"].(bool) {
		fail("Registration is closed right now.", http.StatusForbidden)
		return
	}
	if !s.limit("register:"+s.ip(r), 5, time.Hour) {
		fail("Too many sign-ups from your network. Try again later.", http.StatusTooManyRequests)
		return
	}
	switch {
	case !usernameRe.MatchString(username):
		fail("Usernames are 3–24 letters, digits or underscores.", http.StatusBadRequest)
		return
	case !validEmail(email):
		fail("That e-mail address does not look right.", http.StatusBadRequest)
		return
	case len(pass) < 8 || len(pass) > 200:
		fail("Passwords need at least 8 characters.", http.StatusBadRequest)
		return
	case r.FormValue("website") != "":
		// Honeypot field; bots fill everything.
		http.Redirect(w, r, "/verify?email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	u, err := s.db.CreateUser(r.Context(), username, email, pass, store.RoleReader, false)
	if errors.Is(err, store.ErrTaken) {
		// An abandoned, unverified registration for the same e-mail should
		// not block the person who owns it: let them pick up where they left off.
		if ex, e2 := s.db.UserByEmail(r.Context(), email); e2 == nil && !ex.Verified {
			s.db.SetPassword(r.Context(), ex.ID, pass)
			u, err = ex, nil
		} else {
			fail("That username or e-mail is already taken.", http.StatusConflict)
			return
		}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.sendCode(r.Context(), u.Email, store.PurposeVerify); err != nil && !errors.Is(err, store.ErrCodeThrottled) {
		s.log.Error("send verify code", "err", err)
		fail("We could not send the verification e-mail. Please try again shortly.", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/verify?email="+url.QueryEscape(u.Email), http.StatusSeeOther)
}

func (s *Server) verifyForm(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Verify your e-mail")
	d["NoIndex"] = true
	d["Email"] = r.URL.Query().Get("email")
	s.render(w, r, "verify", d)
}

func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	code := strings.TrimSpace(r.FormValue("code"))
	d := s.data(r, "Verify your e-mail")
	d["NoIndex"], d["Email"] = true, email
	if !s.limit("verify:"+s.ip(r), 20, 10*time.Minute) {
		d["Error"] = "Too many attempts. Request a new code in a few minutes."
		s.renderStatus(w, r, http.StatusTooManyRequests, "verify", d)
		return
	}
	u, err := s.db.UserByEmail(r.Context(), email)
	if err != nil {
		d["Error"] = "No account with that e-mail is waiting for verification."
		s.renderStatus(w, r, http.StatusBadRequest, "verify", d)
		return
	}
	if err := s.db.CheckCode(r.Context(), email, store.PurposeVerify, code); err != nil {
		if errors.Is(err, store.ErrBadCode) {
			d["Error"] = "That code is not right."
		} else {
			d["Error"] = "That code has expired. Request a new one."
		}
		s.renderStatus(w, r, http.StatusBadRequest, "verify", d)
		return
	}
	s.db.ConsumeCode(r.Context(), email, store.PurposeVerify)
	if !u.Verified {
		if err := s.db.MarkVerified(r.Context(), u.ID); err != nil {
			s.fail(w, r, err)
			return
		}
		if s.Settings().NotifyUsers {
			go s.notify("New blog member", u.Username+" <"+u.Email+"> verified their account", "bust_in_silhouette")
		}
	}
	token, err := s.db.CreateSession(r.Context(), u.ID, r.UserAgent(), s.ip(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	s.flash(w, "ok", "Welcome, "+u.Name()+"! Your account is verified.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) verifyResend(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if !s.limit("resend:"+s.ip(r), 5, 10*time.Minute) {
		s.flash(w, "err", "Too many requests. Wait a few minutes.")
		http.Redirect(w, r, "/verify?email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if u, err := s.db.UserByEmail(r.Context(), email); err == nil && !u.Verified {
		if err := s.sendCode(r.Context(), email, store.PurposeVerify); err != nil {
			if errors.Is(err, store.ErrCodeThrottled) {
				s.flash(w, "err", "A code was sent less than a minute ago. Check your inbox and spam folder.")
			} else {
				s.log.Error("resend code", "err", err)
				s.flash(w, "err", "Could not send the e-mail right now.")
			}
			http.Redirect(w, r, "/verify?email="+url.QueryEscape(email), http.StatusSeeOther)
			return
		}
	}
	// Same message whether or not the address exists.
	s.flash(w, "ok", "If that address is waiting for verification, a new code is on its way.")
	http.Redirect(w, r, "/verify?email="+url.QueryEscape(email), http.StatusSeeOther)
}

func (s *Server) forgotForm(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Reset your password")
	d["NoIndex"] = true
	s.render(w, r, "forgot", d)
}

func (s *Server) forgot(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if !s.limit("forgot:"+s.ip(r), 5, 15*time.Minute) {
		s.flash(w, "err", "Too many requests. Wait a few minutes.")
		http.Redirect(w, r, "/forgot", http.StatusSeeOther)
		return
	}
	if u, err := s.db.UserByEmail(r.Context(), email); err == nil && u.Verified {
		if err := s.sendCode(r.Context(), email, store.PurposeReset); err != nil && !errors.Is(err, store.ErrCodeThrottled) {
			s.log.Error("reset code", "err", err)
		}
	}
	http.Redirect(w, r, "/reset?email="+url.QueryEscape(email), http.StatusSeeOther)
}

func (s *Server) resetForm(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Choose a new password")
	d["NoIndex"] = true
	d["Email"] = r.URL.Query().Get("email")
	s.render(w, r, "reset", d)
}

func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	code := strings.TrimSpace(r.FormValue("code"))
	pass := r.FormValue("password")
	d := s.data(r, "Choose a new password")
	d["NoIndex"], d["Email"] = true, email
	if !s.limit("reset:"+s.ip(r), 20, 10*time.Minute) {
		d["Error"] = "Too many attempts. Request a new code in a few minutes."
		s.renderStatus(w, r, http.StatusTooManyRequests, "reset", d)
		return
	}
	if len(pass) < 8 || len(pass) > 200 {
		d["Error"] = "Passwords need at least 8 characters."
		s.renderStatus(w, r, http.StatusBadRequest, "reset", d)
		return
	}
	u, err := s.db.UserByEmail(r.Context(), email)
	if err != nil {
		d["Error"] = "That code is not right."
		s.renderStatus(w, r, http.StatusBadRequest, "reset", d)
		return
	}
	if err := s.db.CheckCode(r.Context(), email, store.PurposeReset, code); err != nil {
		if errors.Is(err, store.ErrBadCode) {
			d["Error"] = "That code is not right."
		} else {
			d["Error"] = "That code has expired. Request a new one."
		}
		s.renderStatus(w, r, http.StatusBadRequest, "reset", d)
		return
	}
	s.db.ConsumeCode(r.Context(), email, store.PurposeReset)
	if err := s.db.SetPassword(r.Context(), u.ID, pass); err != nil {
		s.fail(w, r, err)
		return
	}
	s.db.DeleteUserSessions(r.Context(), u.ID)
	s.flash(w, "ok", "Password changed. You can log in now.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ---- account settings ----

func (s *Server) settingsForm(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Account settings")
	d["NoIndex"] = true
	d["PendingEmail"] = r.URL.Query().Get("pending")
	s.render(w, r, "settings", d)
}

func (s *Server) settingsProfile(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	display := strings.TrimSpace(r.FormValue("display_name"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	site := strings.TrimSpace(r.FormValue("website"))
	if len(display) > 60 || len(bio) > 500 || len(site) > 200 {
		s.flash(w, "err", "Something is too long: name 60, bio 500, website 200 characters at most.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if site != "" {
		if !strings.HasPrefix(site, "http://") && !strings.HasPrefix(site, "https://") {
			site = "https://" + site
		}
		if _, err := url.ParseRequestURI(site); err != nil {
			s.flash(w, "err", "That website URL does not look right.")
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
	}
	if err := s.db.UpdateProfile(r.Context(), u.ID, display, bio, site); err != nil {
		s.fail(w, r, err)
		return
	}
	s.flash(w, "ok", "Profile saved.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) settingsAvatar(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		s.flash(w, "err", "That image is too large (10 MB max).")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if r.FormValue("remove") == "1" {
		s.db.SetAvatar(r.Context(), u.ID, "")
		s.flash(w, "ok", "Avatar removed.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	_, fh, err := r.FormFile("avatar")
	if err != nil {
		s.flash(w, "err", "Choose an image first.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	path, err := s.saveAvatar(fh, u.ID)
	if err != nil {
		msg := "Could not process that image."
		if errors.Is(err, errTooBig) {
			msg = "That image is too large (10 MB max)."
		} else if errors.Is(err, errBadType) {
			msg = "Use a JPEG, PNG, GIF or WebP image."
		}
		s.flash(w, "err", msg)
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := s.db.SetAvatar(r.Context(), u.ID, path); err != nil {
		s.fail(w, r, err)
		return
	}
	s.flash(w, "ok", "Avatar updated.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) settingsPassword(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	cur, next := r.FormValue("current"), r.FormValue("password")
	if _, err := s.db.CheckPassword(r.Context(), u.Username, cur); err != nil {
		s.flash(w, "err", "Your current password was wrong.")
		http.Redirect(w, r, "/settings#password", http.StatusSeeOther)
		return
	}
	if len(next) < 8 || len(next) > 200 {
		s.flash(w, "err", "Passwords need at least 8 characters.")
		http.Redirect(w, r, "/settings#password", http.StatusSeeOther)
		return
	}
	if err := s.db.SetPassword(r.Context(), u.ID, next); err != nil {
		s.fail(w, r, err)
		return
	}
	s.flash(w, "ok", "Password changed.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// settingsEmail starts an address change: a code goes to the NEW address,
// and the change only lands once it is entered.
func (s *Server) settingsEmail(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if !validEmail(email) {
		s.flash(w, "err", "That e-mail address does not look right.")
		http.Redirect(w, r, "/settings#email", http.StatusSeeOther)
		return
	}
	if email == u.Email {
		s.flash(w, "info", "That is already your address.")
		http.Redirect(w, r, "/settings#email", http.StatusSeeOther)
		return
	}
	if _, err := s.db.UserByEmail(r.Context(), email); err == nil {
		s.flash(w, "err", "That address is used by another account.")
		http.Redirect(w, r, "/settings#email", http.StatusSeeOther)
		return
	}
	if !s.limit("email:"+s.ip(r), 5, 15*time.Minute) {
		s.flash(w, "err", "Too many requests. Wait a few minutes.")
		http.Redirect(w, r, "/settings#email", http.StatusSeeOther)
		return
	}
	if err := s.sendCode(r.Context(), email, store.PurposeEmail); err != nil && !errors.Is(err, store.ErrCodeThrottled) {
		s.log.Error("email change code", "err", err)
		s.flash(w, "err", "Could not send the confirmation e-mail.")
		http.Redirect(w, r, "/settings#email", http.StatusSeeOther)
		return
	}
	s.flash(w, "ok", "We sent a code to "+email+". Enter it below to confirm.")
	http.Redirect(w, r, "/settings?pending="+url.QueryEscape(email)+"#email", http.StatusSeeOther)
}

func (s *Server) settingsEmailConfirm(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	code := strings.TrimSpace(r.FormValue("code"))
	back := "/settings?pending=" + url.QueryEscape(email) + "#email"
	if !s.limit("emailc:"+s.ip(r), 20, 10*time.Minute) {
		s.flash(w, "err", "Too many attempts.")
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	if err := s.db.CheckCode(r.Context(), email, store.PurposeEmail, code); err != nil {
		s.flash(w, "err", "That code is wrong or has expired.")
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	if err := s.db.SetEmail(r.Context(), u.ID, email); err != nil {
		s.flash(w, "err", "That address is used by another account.")
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	s.db.ConsumeCode(r.Context(), email, store.PurposeEmail)
	s.flash(w, "ok", "E-mail address updated.")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
