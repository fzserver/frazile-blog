// Package geoip resolves an IP address to a city, region (state), country and
// network provider using local MaxMind-format databases (DB-IP lite). It makes
// no network calls: the .mmdb files are read from disk, so there is no
// third-party API in the request path. With no databases configured it simply
// returns empty fields.
package geoip

import (
	"net"
	"sync"

	"github.com/IncSW/geoip2"
)

// Location is what a lookup yields; any field may be empty.
type Location struct {
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Provider string `json:"provider"`
}

type Resolver struct {
	city *geoip2.CityReader
	asn  *geoip2.ASNReader
	// The readers are safe for concurrent Lookup, but keep a mutex for the
	// (rare) case a future reload swaps them.
	mu sync.RWMutex
}

// Open loads whichever databases are present. Either path may be empty; an
// unreadable or missing file disables that half rather than failing the server.
func Open(cityPath, asnPath string) *Resolver {
	r := &Resolver{}
	if cityPath != "" {
		if c, err := geoip2.NewCityReaderFromFile(cityPath); err == nil {
			r.city = c
		}
	}
	if asnPath != "" {
		if a, err := geoip2.NewASNReaderFromFile(asnPath); err == nil {
			r.asn = a
		}
	}
	return r
}

// Enabled reports whether any lookup data is loaded.
func (r *Resolver) Enabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.city != nil || r.asn != nil
}

// Lookup resolves one IP. Unknown or private addresses come back empty.
func (r *Resolver) Lookup(ipStr string) Location {
	var loc Location
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return loc
	}
	r.mu.RLock()
	city, asn := r.city, r.asn
	r.mu.RUnlock()

	if city != nil {
		if res, err := city.Lookup(ip); err == nil {
			loc.City = pick(res.City.Names)
			if len(res.Subdivisions) > 0 {
				loc.Region = pick(res.Subdivisions[0].Names)
			}
			loc.Country = pick(res.Country.Names)
			if loc.Country == "" {
				loc.Country = res.Country.ISOCode
			}
		}
	}
	if asn != nil {
		if res, err := asn.Lookup(ip); err == nil {
			loc.Provider = res.AutonomousSystemOrganization
		}
	}
	return loc
}

// pick returns the English name if present, else any available name.
func pick(names map[string]string) string {
	if names == nil {
		return ""
	}
	if v, ok := names["en"]; ok {
		return v
	}
	for _, v := range names {
		return v
	}
	return ""
}
