package ratelimit

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientKey returns the key that identifies the client of r for rate
// limiting: its IP address, or its /64 network for IPv6, since one IPv6
// customer usually gets a whole /64 and could otherwise rotate addresses.
//
// Where the client IP comes from depends on trustedHops, the number of
// proxies of ours between the client and the server:
//
//   - 0 (make run, the local stack, tests): the TCP connection's address,
//     r.RemoteAddr. X-Forwarded-For is ignored, because any client can send
//     one with whatever it likes.
//
//   - N >= 1 (Cloud Run): every request reaches the container through
//     Google's infrastructure, so RemoteAddr is Google's, and the client IP
//     is the Nth X-Forwarded-For entry counted from the right. A header with
//     fewer entries, or an entry that is not an IP, falls back to RemoteAddr.
//
// Which X-Forwarded-For entry to trust on Cloud Run. Google's front end
// appends the address of whoever connected to it after any value the
// client sent, and does not check what came before. Google's documentation
// says so for its load balancers: "If the incoming request already includes
// an X-Forwarded-For header, the load balancer appends its values to the
// existing header" (<existing-value>,<client-ip>,<load-balancer-ip>), and
// "The load balancer does not verify any IP addresses that precede
// <client-ip>,<load-balancer-ip> in this header" (Cloud Load Balancing,
// "External Application Load Balancer overview", section "X-Forwarded-For
// header": https://cloud.google.com/load-balancing/docs/https#x-forwarded-for_header).
// The Cloud Run functions page on request headers
// (https://cloud.google.com/functions/docs/reference/headers) describes it
// as "clientIp, proxy1Ip, proxy2Ip" and says the first IP is "generally"
// the client's: true for honest clients, but the first entries are exactly
// the ones a client controls.
//
// So only entries counted from the right are trustworthy. Cloud Run alone
// (on its run.app URL) appends the address that connected to it, the
// client's, so the count is 1; Google documents no count for that case, and
// the first deploy checks it (docs/operations.md). Behind an external
// Application Load Balancer, the balancer appends <client-ip>,
// <load-balancer-ip>, so the count is 2. A count that is too low is safe:
// every client then lands on a proxy's address, shares one bucket and only
// the global limit holds, but nobody can dodge the limit by sending a
// header. A count that is too high is not: it reaches an entry the client
// wrote, so a client could choose its own key. That is why the count comes
// from configuration (TRUSTED_PROXY_HOPS, checked at start-up) and why 2 is
// only for a service whose ingress accepts the load balancer alone.
func ClientKey(r *http.Request, trustedHops int) string {
	var addr netip.Addr
	if trustedHops > 0 {
		addr = forwardedClient(r.Header.Values("X-Forwarded-For"), trustedHops)
	}
	if !addr.IsValid() {
		addr = remoteAddr(r.RemoteAddr)
	}
	if !addr.IsValid() {
		// Not an IP (a unix socket, a test): one shared bucket, still
		// under the global limit.
		return "unknown"
	}
	addr = addr.Unmap() // ::ffff:1.2.3.4 is 1.2.3.4
	if addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().String()
	}
	return addr.String()
}

// forwardedClient returns the X-Forwarded-For entry written by the nearest
// trusted proxy, hops entries from the right. Several header lines count as
// one list, in order (RFC 9110, section 5.3).
func forwardedClient(values []string, hops int) netip.Addr {
	var entries []string
	for _, v := range values {
		for e := range strings.SplitSeq(v, ",") {
			entries = append(entries, strings.TrimSpace(e))
		}
	}
	if len(entries) < hops {
		return netip.Addr{}
	}
	addr, err := netip.ParseAddr(entries[len(entries)-hops])
	if err != nil {
		return netip.Addr{}
	}
	return addr
}

func remoteAddr(raw string) netip.Addr {
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr
}
