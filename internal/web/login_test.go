package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/fzserver/frazile-blog/internal/config"
	"github.com/fzserver/frazile-blog/internal/mail"
	"github.com/fzserver/frazile-blog/internal/stats"
	"github.com/fzserver/frazile-blog/internal/store"
)

// fakeResend stands in for the mail API and remembers the last code mailed.
type fakeResend struct {
	mu   sync.Mutex
	code string
}

func (f *fakeResend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var m struct{ Text string }
	json.NewDecoder(r.Body).Decode(&m)
	if c := regexp.MustCompile(`\b\d{6}\b`).FindString(m.Text); c != "" {
		f.mu.Lock()
		f.code = c
		f.mu.Unlock()
	}
	io.WriteString(w, `{"id":"test"}`)
}

func (f *fakeResend) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.code
}

// The right password never starts a session by itself: every log-in waits
// for the code mailed to the address, in the browser that gave the password.
func TestLogInNeedsTheCode(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Iters = 1000
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := stats.Open(filepath.Join(dir, "stats.db"), "", filepath.Join(dir, "traffic"), nil, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateUser(context.Background(), "asha", "asha@example.com", "correcthorse", store.RoleReader, true); err != nil {
		t.Fatal(err)
	}
	resend := &fakeResend{}
	rs := httptest.NewServer(resend)
	s, err := New(config.Config{PublicURL: "http://blog.test", DataDir: dir}, db, st,
		mail.NewWithEndpoint("re_test", "Blog <blog@example.com>", rs.URL), log)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(func() { srv.Close(); rs.Close(); db.Close() })

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	post := func(path string, form url.Values) *http.Response {
		t.Helper()
		res, err := c.PostForm(srv.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	signedIn := func() bool {
		res, err := c.Get(srv.URL + "/settings")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode == http.StatusOK
	}

	if res := post("/login", url.Values{"login": {"asha"}, "password": {"nope-nope"}}); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad password: %d", res.StatusCode)
	}
	res := post("/login", url.Values{"login": {"asha"}, "password": {"correcthorse"}, "next": {"/settings"}})
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login/code?next=%2Fsettings" {
		t.Fatalf("password step: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if signedIn() {
		t.Fatal("logged in on the password alone")
	}
	code := resend.last()
	if code == "" {
		t.Fatal("no code was mailed")
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if res := post("/login/code", url.Values{"code": {wrong}}); res.StatusCode != http.StatusBadRequest || signedIn() {
		t.Fatalf("wrong code: %d", res.StatusCode)
	}

	// The code is no good in another browser, whatever the form claims.
	mine := c.Jar
	c.Jar, _ = cookiejar.New(nil)
	if res := post("/login/code", url.Values{"email": {"asha@example.com"}, "code": {code}}); res.Header.Get("Location") != "/login" || signedIn() {
		t.Fatalf("code without the cookie: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	c.Jar = mine

	if res := post("/login/code", url.Values{"code": {code}, "next": {"/settings"}}); res.Header.Get("Location") != "/settings" {
		t.Fatalf("right code: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if !signedIn() {
		t.Fatal("not logged in after the code")
	}
}
