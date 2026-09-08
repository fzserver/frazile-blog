package stats

import "strings"

// A deliberately small User-Agent classifier. Real UA parsing is a bottomless
// pit; this covers the browsers, OSes and form factors that actually show up
// and labels everything else "Other", which is all a traffic dashboard needs.

func classify(ua string) (browser, os, device string) {
	l := strings.ToLower(ua)
	if ua == "" {
		return "Unknown", "Unknown", "unknown"
	}

	// Order matters: Edge/Brave/Opera all embed "chrome"; iOS browsers all
	// embed "safari"; so the more specific token wins first.
	switch {
	case strings.Contains(l, "edg/") || strings.Contains(l, "edga/") || strings.Contains(l, "edgios/"):
		browser = "Edge"
	case strings.Contains(l, "opr/") || strings.Contains(l, "opera"):
		browser = "Opera"
	case strings.Contains(l, "samsungbrowser"):
		browser = "Samsung Internet"
	case strings.Contains(l, "firefox") || strings.Contains(l, "fxios"):
		browser = "Firefox"
	case strings.Contains(l, "chrome") || strings.Contains(l, "crios"):
		browser = "Chrome"
	case strings.Contains(l, "safari"):
		browser = "Safari"
	case strings.Contains(l, "bot") || strings.Contains(l, "spider") || strings.Contains(l, "crawl"):
		browser = "Bot"
	case strings.Contains(l, "curl") || strings.Contains(l, "wget") || strings.Contains(l, "python") || strings.Contains(l, "go-http"):
		browser = "Script"
	default:
		browser = "Other"
	}

	switch {
	case strings.Contains(l, "android"):
		os = "Android"
	case strings.Contains(l, "iphone") || strings.Contains(l, "ipad") || strings.Contains(l, "ipod"):
		os = "iOS"
	case strings.Contains(l, "windows"):
		os = "Windows"
	case strings.Contains(l, "mac os x") || strings.Contains(l, "macintosh"):
		os = "macOS"
	case strings.Contains(l, "cros"):
		os = "ChromeOS"
	case strings.Contains(l, "linux"):
		os = "Linux"
	default:
		os = "Other"
	}

	switch {
	case strings.Contains(l, "ipad") || (strings.Contains(l, "android") && !strings.Contains(l, "mobile")):
		device = "tablet"
	case strings.Contains(l, "mobile") || strings.Contains(l, "iphone") || strings.Contains(l, "android"):
		device = "mobile"
	case browser == "Bot" || browser == "Script":
		device = "bot"
	default:
		device = "desktop"
	}
	return browser, os, device
}

// IsBot reports whether a user agent looks like a crawler or a script.
func IsBot(ua string) bool {
	b, _, _ := classify(ua)
	return b == "Bot" || b == "Script"
}
