// Package network detects the host's public addresses and chooses a plausible
// masquerade / handshake domain based on the server's country.
package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// domainByCountry maps an ISO country code to a locally-plausible HTTPS site
// used as the Reality/ShadowTLS handshake target (mirrors legacy defaults.conf).
var domainByCountry = map[string]string{
	"TW": "www.apple.com",
	"HK": "www.apple.com",
	"NG": "unn.edu.ng",
	"JP": "www.tms-e.co.jp",
	"US": "www.thewaltdisneycompany.com",
	"NL": "nl.servutech.com",
	"DE": "www.mediamarkt.de",
}

const defaultDomain = "www.apple.com"

var ipv4Providers = []string{
	"https://api.ipify.org",
	"https://ipv4.icanhazip.com",
	"https://ifconfig.me/ip",
}

var ipv6Providers = []string{
	"https://api6.ipify.org",
	"https://ipv6.icanhazip.com",
}

// Addresses is the result of public-IP detection.
type Addresses struct {
	IPv4 string // empty if none
	IPv6 string // empty if none
}

// Detect probes for the host's public IPv4 and IPv6 addresses.
func Detect(ctx context.Context) Addresses {
	return Addresses{
		IPv4: firstValidIP(ctx, ipv4Providers, false),
		IPv6: firstValidIP(ctx, ipv6Providers, true),
	}
}

// SelectDomain returns a masquerade domain appropriate to the server's country,
// falling back to the default when geolocation fails.
func SelectDomain(ctx context.Context) string {
	cc := country(ctx)
	if d, ok := domainByCountry[strings.ToUpper(cc)]; ok {
		return d
	}
	return defaultDomain
}

func country(ctx context.Context) string {
	body, err := httpGet(ctx, "https://ipapi.co/country/", 8*time.Second)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(body)
}

func firstValidIP(ctx context.Context, providers []string, wantV6 bool) string {
	for _, p := range providers {
		body, err := httpGet(ctx, p, 8*time.Second)
		if err != nil {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(body))
		if ip == nil {
			continue
		}
		isV6 := ip.To4() == nil
		if isV6 == wantV6 {
			return ip.String()
		}
	}
	return ""
}

func httpGet(ctx context.Context, url string, timeout time.Duration) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
