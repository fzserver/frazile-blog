// Package config reads the service configuration from the environment. Every
// knob has a default that works for a local run; the compose file overrides
// what differs in production.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr      string // listen address
	PublicURL string // https://blog.frazile.com -- canonical URLs, cookies' Secure flag, CSRF origin
	DataDir   string // writable: blog.db, stats.db, traffic/ journal
	MediaDir  string // writable: uploaded images/videos/avatars

	// Mail (Resend). Empty key = mail off, which also disables registration
	// because accounts cannot be verified.
	ResendKey   string
	MailFrom    string
	MailReplyTo string

	// Bootstrap admin, created once if no admin account exists.
	AdminUser  string
	AdminEmail string
	AdminPass  string

	// Traffic stats: geoip databases + optional VPN CIDR list.
	GeoCityDB string
	GeoASNDB  string
	VPNFile   string

	// Optional ntfy topic URL for new-comment / new-user pings.
	NtfyURL string

	MaxImageMB int
	MaxVideoMB int

	// TrustProxy: read the client IP from CF-Connecting-IP / X-Forwarded-For.
	// True in production (behind the cloudflared tunnel), false for a bare run.
	TrustProxy bool
	LogLevel   string
}

func FromEnv() (Config, error) {
	c := Config{
		Addr:        env("ADDR", ":8080"),
		PublicURL:   strings.TrimRight(env("PUBLIC_URL", "http://localhost:8080"), "/"),
		DataDir:     env("DATA_DIR", "./data"),
		MediaDir:    env("MEDIA_DIR", ""),
		ResendKey:   os.Getenv("RESEND_API_KEY"),
		MailFrom:    os.Getenv("MAIL_FROM"),
		MailReplyTo: os.Getenv("MAIL_REPLY_TO"),
		AdminUser:   env("ADMIN_USERNAME", "admin"),
		AdminEmail:  os.Getenv("ADMIN_EMAIL"),
		AdminPass:   os.Getenv("ADMIN_PASSWORD"),
		GeoCityDB:   os.Getenv("GEOIP_CITY_DB"),
		GeoASNDB:    os.Getenv("GEOIP_ASN_DB"),
		VPNFile:     os.Getenv("VPN_CIDR_FILE"),
		NtfyURL:     os.Getenv("NTFY_URL"),
		MaxImageMB:  envInt("MAX_IMAGE_MB", 25),
		MaxVideoMB:  envInt("MAX_VIDEO_MB", 1024),
		TrustProxy:  envBool("TRUST_PROXY", true),
		LogLevel:    env("LOG_LEVEL", "info"),
	}
	if c.MediaDir == "" {
		c.MediaDir = c.DataDir + "/media"
	}
	if _, err := url.Parse(c.PublicURL); err != nil {
		return c, fmt.Errorf("PUBLIC_URL: %w", err)
	}
	return c, nil
}

// Host is the hostname part of PublicURL (for same-site referrer checks).
func (c Config) Host() string {
	u, _ := url.Parse(c.PublicURL)
	return u.Host
}

func (c Config) Secure() bool { return strings.HasPrefix(c.PublicURL, "https://") }

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	switch strings.ToLower(os.Getenv(k)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
