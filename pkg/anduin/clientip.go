package anduin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// ParseTrustedProxies parses IP addresses and CIDR ranges (for example
// "10.0.0.0/8" or "127.0.0.1") into prefixes for RealIP. Blank values are
// skipped.
func ParseTrustedProxies(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return nil, fmt.Errorf("anduin: trusted proxy %q: %w", value, err)
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return nil, fmt.Errorf("anduin: trusted proxy %q: %w", value, err)
		}
		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}

// RealIP resolves the client address for ClientIP. When the direct peer is in
// trusted, X-Forwarded-For is walked from the right, skipping trusted hops, and
// the first untrusted address is the client. Forwarding headers from an
// untrusted peer are ignored, so with no trusted proxies ClientIP is always the
// peer address.
func RealIP(trusted ...netip.Prefix) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			if state := stateFrom(ctx); state != nil && state.request != nil {
				state.clientIP = resolveClientIP(state.request, trusted)
			}
			return next(ctx)
		}
	}
}

// ClientIP returns the client address for the request on ctx: the address
// RealIP resolved, or the host part of RemoteAddr without it.
func ClientIP(ctx context.Context) string {
	state := stateFrom(ctx)
	if state == nil {
		return ""
	}
	if state.clientIP != "" {
		return state.clientIP
	}
	if state.request == nil {
		return ""
	}
	return peerIP(state.request)
}

func resolveClientIP(request *http.Request, trusted []netip.Prefix) string {
	peer, ok := parseIP(remoteHost(request.RemoteAddr))
	if !ok || !isTrusted(peer, trusted) {
		return peerIP(request)
	}

	client := peer
	hops := request.Header.Values("X-Forwarded-For")
	for _, hop := range slices.Backward(hops) {
		entries := strings.Split(hop, ",")
		for _, entry := range slices.Backward(entries) {
			addr, ok := parseIP(entry)
			if !ok {
				return client.String()
			}
			client = addr
			if !isTrusted(addr, trusted) {
				return client.String()
			}
		}
	}
	return client.String()
}

func peerIP(request *http.Request) string {
	host := remoteHost(request.RemoteAddr)
	if addr, ok := parseIP(host); ok {
		return addr.String()
	}
	return host
}

func remoteHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func parseIP(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return addrPort.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
