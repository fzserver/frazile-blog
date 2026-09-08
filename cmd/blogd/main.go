// Command blogd serves the blog: public pages, accounts, comments and the
// admin panel, from one SQLite file plus a media directory.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // TZ works in the scratch image

	"github.com/fzserver/frazile-blog/internal/config"
	"github.com/fzserver/frazile-blog/internal/geoip"
	"github.com/fzserver/frazile-blog/internal/mail"
	"github.com/fzserver/frazile-blog/internal/stats"
	"github.com/fzserver/frazile-blog/internal/store"
	"github.com/fzserver/frazile-blog/internal/web"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the running server and exit 0 if healthy")
	flag.Parse()

	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *healthcheck {
		os.Exit(probe(cfg.Addr))
	}

	level := slog.LevelInfo
	if strings.EqualFold(cfg.LogLevel, "debug") {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	for _, dir := range []string{cfg.DataDir, cfg.MediaDir, filepath.Join(cfg.DataDir, "traffic")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Error("mkdir", "dir", dir, "err", err)
			os.Exit(1)
		}
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "blog.db"))
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	geo := geoip.Open(cfg.GeoCityDB, cfg.GeoASNDB)
	st, err := stats.Open(filepath.Join(cfg.DataDir, "stats.db"), cfg.VPNFile, filepath.Join(cfg.DataDir, "traffic"), geo, log)
	if err != nil {
		log.Error("open stats", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	mailer := mail.New(cfg.ResendKey, cfg.MailFrom)
	if mailer == nil {
		log.Warn("mail is off: RESEND_API_KEY / MAIL_FROM unset; registration and password reset are disabled")
	}

	ctx := context.Background()
	if !db.AdminExists(ctx) {
		if cfg.AdminEmail != "" && cfg.AdminPass != "" {
			if _, err := db.CreateUser(ctx, cfg.AdminUser, cfg.AdminEmail, cfg.AdminPass, store.RoleAdmin, true); err != nil {
				log.Error("create admin", "err", err)
			} else {
				log.Info("created admin account", "username", cfg.AdminUser)
			}
		} else {
			log.Warn("no admin account exists and ADMIN_EMAIL/ADMIN_PASSWORD are unset")
		}
	}

	srv, err := web.New(cfg, db, st, mailer, log)
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}
	hs := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Long enough for a large video upload over a slow link.
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	go func() {
		log.Info("listening", "addr", cfg.Addr, "public", cfg.PublicURL, "geoip", geo.Enabled(), "mail", mailer != nil)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("serve", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hs.Shutdown(sctx)
	log.Info("stopped")
}

func probe(addr string) int {
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 4 * time.Second}
	res, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		return 1
	}
	return 0
}
