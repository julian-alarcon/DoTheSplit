package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// TrustedProxies reproduces gin's SetTrustedProxies semantics: it decides
// whether to believe X-Forwarded-For based on whether the immediate peer
// (RemoteAddr) is a configured proxy. Build it once at startup and share it
// across the rate limiter, access logger, and any handler that audits a client
// IP. With an empty set nothing is trusted, so a forged X-Forwarded-For is
// ignored and the real RemoteAddr is always used.
type TrustedProxies struct {
	nets []*net.IPNet
	ips  []net.IP
}

// NewTrustedProxies parses a list of proxy entries, each either a CIDR
// ("10.0.0.0/8") or a bare IP ("192.168.1.1").
func NewTrustedProxies(entries []string) (*TrustedProxies, error) {
	tp := &TrustedProxies{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(e); err == nil {
			tp.nets = append(tp.nets, ipnet)
			continue
		}
		ip := net.ParseIP(e)
		if ip == nil {
			return nil, fmt.Errorf("invalid trusted proxy %q", e)
		}
		tp.ips = append(tp.ips, ip)
	}
	return tp, nil
}

func (tp *TrustedProxies) isTrusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range tp.nets {
		if n.Contains(ip) {
			return true
		}
	}
	for _, t := range tp.ips {
		if t.Equal(ip) {
			return true
		}
	}
	return false
}

// ClientIP returns the best-guess originating client IP for r. If the immediate
// peer is not a trusted proxy, its address wins (X-Forwarded-For is ignored). If
// it is trusted, the rightmost non-trusted hop in X-Forwarded-For is returned.
func (tp *TrustedProxies) ClientIP(r *http.Request) string {
	remote := remoteHost(r.RemoteAddr)
	if !tp.isTrusted(net.ParseIP(remote)) {
		return remote
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remote
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(parts[i])
		ip := net.ParseIP(hop)
		if ip == nil {
			continue
		}
		if !tp.isTrusted(ip) {
			return hop
		}
	}
	return remote
}

// remoteHost strips the port from a "host:port" RemoteAddr, falling back to the
// raw value when it has no port (already a bare host).
func remoteHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
