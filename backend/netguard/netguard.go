// Package netguard is the one implementation of the outbound SSRF guard. Every feature
// that fetches a user-supplied URL (webhooks, CalDAV, CardDAV, Monica, photo URLs)
// dials through DialContext, so the address that is checked is the address that is used:
// redirects and DNS rebinding cannot reach an internal host.
//
// Not guarded, on purpose: the OIDC issuer and the SMTP host. Both are set by the
// administrator in the environment, not by users, and are often internal services.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// Errors returned by DialContext when callers pass no sentinels of their own.
var (
	ErrUnreachable    = errors.New("host could not be resolved")
	ErrPrivateAddress = errors.New("address is private, loopback or link-local")
)

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// IsBlockedIP reports whether ip is an address that must not be reached when private
// URLs are blocked: loopback, private (RFC 1918, unique local), link-local (this
// includes the cloud metadata address 169.254.169.254), multicast, the unspecified
// address (0.0.0.0 reaches the local host on Linux) and carrier-grade NAT
// (100.64.0.0/10, where some clouds put their metadata service). IPv4-mapped IPv6
// addresses are judged as the IPv4 address they contain.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		if cgnat.Contains(ip) || ip.Equal(net.IPv4bcast) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

// NewDialContext returns a DialContext that resolves the host, drops blocked addresses
// and dials the remaining ones by IP (trying each in turn). It returns errPrivate when
// no usable address is left and errUnreachable when the name does not resolve; callers
// pass their feature's sentinel errors so controllers can map failures.
func NewDialContext(errUnreachable, errPrivate error) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("%w: could not resolve host", errUnreachable)
		}
		var safe []net.IP
		for _, ip := range ips {
			if !IsBlockedIP(ip) {
				safe = append(safe, ip)
			}
		}
		if len(safe) == 0 {
			return nil, errPrivate
		}
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 10 * time.Second}
		var lastErr error
		for _, ip := range safe {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// DialContext is NewDialContext with this package's sentinel errors.
var DialContext = NewDialContext(ErrUnreachable, ErrPrivateAddress)
