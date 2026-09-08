package stats

import (
	"bufio"
	"net"
	"net/netip"
	"os"
	"sort"
)

// vpnRanges is a set of CIDR blocks considered "VPN/datacenter". Real proxy
// intelligence needs a paid data provider; this instead matches against a
// locally-loaded list of hosting/VPN network ranges. No file, no detection --
// the flag is simply false, never a guess.
type vpnRanges struct {
	prefixes []netip.Prefix
}

// loadVPNRanges reads one CIDR per line (blank lines and #-comments ignored).
// A missing file is fine: VPN detection is then just disabled.
func loadVPNRanges(path string) *vpnRanges {
	v := &vpnRanges{}
	if path == "" {
		return v
	}
	f, err := os.Open(path)
	if err != nil {
		return v
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := indexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = trim(line)
		if line == "" {
			continue
		}
		if p, err := netip.ParsePrefix(line); err == nil {
			v.prefixes = append(v.prefixes, p.Masked())
		} else if ip, err := netip.ParseAddr(line); err == nil {
			// A bare address counts as a /32 or /128.
			bits := 32
			if ip.Is6() {
				bits = 128
			}
			v.prefixes = append(v.prefixes, netip.PrefixFrom(ip, bits))
		}
	}
	// Sorted by address so lookups can short-circuit; linear scan is fine for
	// the modest lists a single site curates.
	sort.Slice(v.prefixes, func(i, j int) bool {
		return v.prefixes[i].Addr().Less(v.prefixes[j].Addr())
	})
	return v
}

func (v *vpnRanges) contains(ipStr string) bool {
	if len(v.prefixes) == 0 || ipStr == "" {
		return false
	}
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		// Fall back to net.ParseIP for oddities, then re-encode.
		if p := net.ParseIP(ipStr); p != nil {
			if a, ok := netip.AddrFromSlice(p); ok {
				ip = a.Unmap()
			} else {
				return false
			}
		} else {
			return false
		}
	}
	ip = ip.Unmap()
	for _, p := range v.prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func (v *vpnRanges) enabled() bool { return len(v.prefixes) > 0 }

// small dependency-free helpers so this file needs no strings import churn
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// writeFile is a tiny test helper kept out of the _test file so it can be used
// by the exported-less test without importing os there twice.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
