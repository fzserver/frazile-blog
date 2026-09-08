package stats

import (
	"net/url"
	"strings"
)

// Attribution is where a visit came from, reduced to the four things worth
// counting. It is derived once, when the visit is recorded, and stored on the
// row -- the same reasoning as geolocation: the raw Referer header may be
// long, noisy and full of query strings nobody wants kept, while the parts
// that answer "which channel is working" are short and stable.
type Attribution struct {
	// Referrer is the bare hostname of the referring page ("" for a direct
	// hit or a stripped referrer). Never the full URL: the path of the page
	// someone came from is somebody else's business, and on a search engine
	// it would carry the query.
	Referrer string `json:"referrer"`
	// Source is the channel, normalised: "google", "reddit", "x", "direct",
	// or the hostname when it is nothing known. utm_source wins when present,
	// because that is the campaign's own claim about itself.
	Source string `json:"source"`
	// Medium is how they arrived: organic, social, referral, direct, or
	// whatever utm_medium says (email, cpc, ...).
	Medium string `json:"medium"`
	// Campaign is utm_campaign, for telling two posts on the same site apart.
	Campaign string `json:"campaign"`
	// Internal reports that the Referer was this site itself -- someone
	// clicking from one of our pages to another. Not stored: it exists so
	// the caller can tell internal navigation from a fresh arrival, which
	// is the whole difference between a landing and a second pageview.
	Internal bool `json:"-"`
}

const (
	MediumDirect   = "direct"
	MediumOrganic  = "organic"
	MediumSocial   = "social"
	MediumReferral = "referral"
)

// searchHosts maps a brand label to the engine name (see matchHost), so
// google.com, google.co.in, google.de and www.google.com all land on
// "google" while google.evil.example does not.
var searchHosts = map[string]string{
	"google.":     "google",
	"bing.":       "bing",
	"duckduckgo.": "duckduckgo",
	"yandex.":     "yandex",
	"baidu.":      "baidu",
	"ecosia.":     "ecosia",
	"brave.":      "brave",
	"yahoo.":      "yahoo",
	"startpage.":  "startpage",
	"qwant.":      "qwant",
}

// socialHosts is the same idea for the places a wallpaper link actually gets
// posted. Kept deliberately short: anything not listed is a plain referral,
// which is the honest answer rather than a guess.
var socialHosts = map[string]string{
	"reddit.":              "reddit",
	"redd.it":              "reddit",
	"x.com":                "x",
	"twitter.":             "x",
	"t.co":                 "x",
	"facebook.":            "facebook",
	"instagram.":           "instagram",
	"pinterest.":           "pinterest",
	"tumblr.":              "tumblr",
	"youtube.":             "youtube",
	"youtu.be":             "youtube",
	"t.me":                 "telegram",
	"telegram.":            "telegram",
	"whatsapp.":            "whatsapp",
	"linkedin.":            "linkedin",
	"lnkd.in":              "linkedin",
	"discord.":             "discord",
	"news.ycombinator.com": "hackernews",
	"producthunt.":         "producthunt",
	"mastodon.":            "mastodon",
	"bsky.":                "bluesky",
	"threads.":             "threads",
}

// Attribute derives the attribution for one request from the Referer header
// and the landing URL's query string. selfHost is this site's own hostname:
// a referrer matching it is internal navigation, not a new arrival, and is
// reported as direct rather than as the site referring itself.
func Attribute(referer, selfHost string, q url.Values) Attribution {
	a := Attribution{
		Source:   strings.ToLower(strings.TrimSpace(q.Get("utm_source"))),
		Medium:   strings.ToLower(strings.TrimSpace(q.Get("utm_medium"))),
		Campaign: strings.TrimSpace(q.Get("utm_campaign")),
	}
	a.Source = truncate(a.Source, 64)
	a.Medium = truncate(a.Medium, 32)
	a.Campaign = truncate(a.Campaign, 64)

	host := refererHost(referer)
	switch {
	case host == "":
	case sameSite(host, selfHost):
		a.Internal = true
	default:
		a.Referrer = host
	}

	// A tagged link describes itself; trust it and stop.
	if a.Source != "" {
		if a.Medium == "" {
			a.Medium = MediumReferral
		}
		return a
	}
	if a.Referrer == "" {
		if a.Medium == "" {
			a.Medium = MediumDirect
		}
		a.Source = MediumDirect
		return a
	}
	if name, ok := matchHost(a.Referrer, searchHosts); ok {
		a.Source = name
		if a.Medium == "" {
			a.Medium = MediumOrganic
		}
		return a
	}
	if name, ok := matchHost(a.Referrer, socialHosts); ok {
		a.Source = name
		if a.Medium == "" {
			a.Medium = MediumSocial
		}
		return a
	}
	a.Source = a.Referrer
	if a.Medium == "" {
		a.Medium = MediumReferral
	}
	return a
}

// refererHost reduces a Referer header to a lowercase hostname with any
// leading "www." dropped. Anything unparseable yields "", which reads as
// direct -- a malformed header is not worth a row of its own.
func refererHost(referer string) string {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Host == "" {
		return ""
	}
	return truncate(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), 128)
}

// sameSite reports whether a referring host is us: the site's own hostname or
// any subdomain of it. Without this every click inside the site would be
// counted as a referral from the site.
func sameSite(host, self string) bool {
	self = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(self)), "www.")
	if self == "" {
		return false
	}
	return host == self || strings.HasSuffix(host, "."+self)
}

// matchHost looks a host up in one of the tables. A key ending in "." matches
// the host's brand label -- "google." is google.com, google.co.in and
// news.google.com, but NOT google.evil.example, which merely begins with the
// same word. Any other key must equal the host or be a suffix of it at a
// label boundary.
//
// The distinction matters because Referer is set by whoever links to us: a
// prefix match would let any domain registration put itself in the reports
// under Google's name.
func matchHost(host string, table map[string]string) (string, bool) {
	brand := brandLabel(host)
	for key, name := range table {
		if strings.HasSuffix(key, ".") {
			if brand != "" && brand == strings.TrimSuffix(key, ".") {
				return name, true
			}
			continue
		}
		if host == key || strings.HasSuffix(host, "."+key) {
			return name, true
		}
	}
	return "", false
}

// registryLabels are second-level labels that are part of the registry rather
// than the name someone registered, so "google" is still the brand in
// google.co.uk. Not a full public-suffix list -- just the shapes a blog
// site's referrers actually arrive in.
var registryLabels = map[string]bool{
	"co": true, "com": true, "net": true, "org": true, "edu": true,
	"gov": true, "ac": true, "or": true, "ne": true, "in": true,
}

// brandLabel is the registered name in a hostname: the label just below the
// public suffix. google.com, www.google.com and google.co.in all yield
// "google"; google.evil.example yields "evil".
func brandLabel(host string) string {
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return ""
	}
	labels = labels[:len(labels)-1] // drop the TLD
	if len(labels) >= 2 && registryLabels[labels[len(labels)-1]] {
		labels = labels[:len(labels)-1] // drop co/com/... in co.uk, com.au
	}
	return labels[len(labels)-1]
}

// truncate bounds a stored field. Every attribution field comes from a header
// or a query string, so all of them are attacker-controlled and none of them
// is worth an unbounded column.
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
