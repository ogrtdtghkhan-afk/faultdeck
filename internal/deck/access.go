package deck

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/url"
	"strings"
)

func secretEqual(a, b string) bool {
	aHash, bHash := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aHash[:], bHash[:]) == 1
}

// An explicit public origin is trusted without trusting forwarded headers.
// Loopback hostnames remain usable for local access and SSH tunnels.
func (d *Deck) controlOrigin(host string) (string, bool) {
	configured, err := url.Parse(d.config.AdminURL)
	if err == nil && configured.Host != "" && strings.EqualFold(host, configured.Host) {
		return configured.Scheme + "://" + configured.Host, true
	}
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	hostname = strings.TrimSuffix(strings.ToLower(hostname), ".")
	ip := net.ParseIP(hostname)
	if hostname == "localhost" || (ip != nil && ip.IsLoopback()) {
		return "http://" + host, true
	}
	return "", false
}
