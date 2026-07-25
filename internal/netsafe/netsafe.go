// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package netsafe validates URLs and resolved addresses for SSRF-safe fetches.
package netsafe

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Options controls scheme and host policy for a fetch target.
type Options struct {
	AllowInsecureHTTP bool
	// ExtraAllowedHosts permits specific hostnames (lowercase) that resolve to
	// otherwise-blocked addresses (for administrator internal-host allowlists).
	ExtraAllowedHosts map[string]struct{}
}

// ValidateURL parses and checks scheme/host policy before DNS.
func ValidateURL(raw string, opt Options) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Opaque != "" || u.User != nil {
		return nil, fmt.Errorf("url must not include opaque data or userinfo")
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
		// ok
	case "http":
		if !opt.AllowInsecureHTTP {
			return nil, fmt.Errorf("insecure http requires allowInsecureHTTP")
		}
	case "file", "ftp", "gopher", "data", "javascript", "blob", "":
		return nil, fmt.Errorf("scheme %q is not allowed", u.Scheme)
	default:
		return nil, fmt.Errorf("scheme %q is not allowed", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if isBlockedHostname(host) && !hostAllowlisted(host, opt) {
		return nil, fmt.Errorf("host %q is blocked", host)
	}
	return u, nil
}

// ValidateResolvedHost checks DNS results (or literal IP) against private ranges.
func ValidateResolvedHost(host string, opt Options) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if hostAllowlisted(host, opt) {
		return nil
	}
	if isBlockedHostname(host) {
		return fmt.Errorf("host %q is blocked", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("address %s is blocked", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve %s: no addresses", host)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return fmt.Errorf("resolved address %s for %s is blocked", ip, host)
		}
	}
	return nil
}

// IsBlockedIP reports whether ip is loopback, private, link-local, multicast, or metadata.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// IPv4 cloud metadata / link-local extras
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 169 && v4[1] == 254 {
			return true
		}
		// 0.0.0.0/8
		if v4[0] == 0 {
			return true
		}
	}
	// Unique local IPv6 fc00::/7
	if ip.To4() == nil && len(ip) == net.IPv6len {
		if ip[0]&0xfe == 0xfc {
			return true
		}
	}
	return false
}

func isBlockedHostname(host string) bool {
	switch host {
	case "localhost", "localhost.", "metadata", "metadata.google.internal":
		return true
	}
	if strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	return false
}

func hostAllowlisted(host string, opt Options) bool {
	if len(opt.ExtraAllowedHosts) == 0 {
		return false
	}
	_, ok := opt.ExtraAllowedHosts[strings.ToLower(host)]
	return ok
}

// PrefixAllowed reports whether target is under at least one allowed prefix
// (exact string prefix match on the full URL).
func PrefixAllowed(target string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, p := range allowed {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(target, p) {
			return true
		}
	}
	return false
}

// SameHost reports whether a and b share the same hostname (case-insensitive).
func SameHost(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Hostname(), b.Hostname())
}
