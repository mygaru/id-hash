package httpauth

import (
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"gitlab.adtelligent.com/common/shared/log"
	"net"
	"strings"
)

var (
	httpServerAllowedRemoteIPs = flag.String(
		"httpAuthAllowedRemoteIPs",
		"127.0.0.1,::1",
		"List of allowed remote ips. If non-empty, the server refuses connections from ips other than listed here",
	)
	httpServerAllowCheckViaHeader = flag.Bool(
		"httpAuthAllowCheckViaHeader",
		false,
		"If true, the server will check the X-Forwarded-For header for the client's IP address",
	)
)

var allowedIPsMap = make(map[string]struct{})
var allowedNets = make([]*net.IPNet, 0)

func InitAllowedIPs() {
	if len(*httpServerAllowedRemoteIPs) == 0 {
		return
	}

	if err := ParseAllowedIPs(*httpServerAllowedRemoteIPs, allowedIPsMap, &allowedNets); err != nil {
		log.Fatalf("Error parsing httpAuthAllowedRemoteIPs: %v", err)
	}
}

func ParseAllowedIPs(allowedIPs string, allowedIPsMap map[string]struct{}, allowedNets *[]*net.IPNet) error {
	*allowedNets = (*allowedNets)[:0]

	for _, ipStr := range strings.Split(allowedIPs, ",") {
		ipStr = strings.TrimSpace(ipStr)

		if strings.Contains(ipStr, "/") {
			_, subnet, err := net.ParseCIDR(ipStr)
			if err != nil {
				return fmt.Errorf("invalid subnet: %q", ipStr)
			}
			*allowedNets = append(*allowedNets, subnet)
		} else {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return fmt.Errorf("invalid IP: %q", ipStr)
			}
			allowedIPsMap[ipStr] = struct{}{}
		}
	}

	return nil
}

func IsAuthorized(ctx *fasthttp.RequestCtx) bool {
	if len(allowedIPsMap) == 0 && len(allowedNets) == 0 {
		return true
	}

	if CheckIP(ctx.RemoteIP(), allowedIPsMap, allowedNets) {
		return true
	}

	if !*httpServerAllowCheckViaHeader {
		return false
	}

	for _, ipStr := range strings.Split(string(ctx.Request.Header.Peek("X-Forwarded-For")), ",") {
		if CheckIP(net.ParseIP(ipStr), allowedIPsMap, allowedNets) {
			return true
		}
	}

	return false
}

func CheckIP(ip net.IP, allowedIPsMap map[string]struct{}, allowedNets []*net.IPNet) bool {
	if ip == nil {
		return false
	}

	_, ok := allowedIPsMap[ip.String()]
	if ok {
		return true
	}

	for _, subnet := range allowedNets {
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}
