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
// Where the client IP comes from depends on how requests reach the server:
//
//   - Directly (make run, the local stack, tests): the TCP connection's
//     address, r.RemoteAddr. X-Forwarded-For is ignored, because any client
//     can send one with whatever it likes.
//
//   - On Cloud Run (behindCloudRun): every request reaches the container
//     through Google's front end, so RemoteAddr is Google's, and the client
//     IP is in X-Forwarded-For.
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
// So only entries counted from the right are trustworthy, and we take the
// last one. MeuRPG runs on Cloud Run with no load balancer in front
// (docs/operacao.md). For that case Google documents no count; the front
// end appends the address that connected to it, the client's, and there is
// no forwarding rule address to add. If that ever proves wrong (the first
// deploy checks it, see docs/operacao.md), the failure is safe: every
// client shares one bucket and only the global limit holds, but nobody can
// dodge the limit by sending a header. If a load balancer is ever put in
// front, it appends <client-ip>,<load-balancer-ip>, and
// cloudRunTrustedHops must become 2.
func ClientKey(r *http.Request, behindCloudRun bool) string {
	var addr netip.Addr
	if behindCloudRun {
		addr = forwardedClient(r.Header.Values("X-Forwarded-For"), cloudRunTrustedHops)
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

// cloudRunTrustedHops is how many X-Forwarded-For entries, counted from
// the right, Google's infrastructure wrote in front of a Cloud Run service
// without a load balancer; the client's IP is the leftmost of them.
const cloudRunTrustedHops = 1

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
