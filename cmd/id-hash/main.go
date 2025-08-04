package main

import (
	"flag"
	"fmt"
	"github.com/mygaru/id-hash/cmd/id-hash/internal/pim"
	"github.com/valyala/fasthttp"
	"github.com/vharitonsky/iniflags"
	"log"
	"net"
	"strings"
	"time"
)

var (
	httpAuthAllowedRemoteIPs = flag.String("httpAuthAllowedRemoteIPs", "127.0.0.1/24", "List of IPs and subnets allowed to send requests")
	httpServerName           = flag.String("httpServerName", "MyGaru ID HASH", "Name of the server")
	httpServerListenAddr     = flag.String("httpServerListenAddr", ":8080", "Listen port for http server")
)

var (
	allowedIPsMap = make(map[string]struct{})
	allowedNets   = make([]*net.IPNet, 0)
)

func main() {
	iniflags.Parse()
	logAllFlags()

	runServer(requestHandler)
}

func runServer(handler func(ctx *fasthttp.RequestCtx)) {
	if err := parseAllowedIPs(*httpAuthAllowedRemoteIPs, allowedIPsMap, &allowedNets); err != nil {
		log.Fatalf("Error parsing httpAuthAllowedRemoteIPs: %v", err)
	}

	s := &fasthttp.Server{
		Handler:      handler,
		Name:         *httpServerName,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp4", *httpServerListenAddr)
	if err != nil {
		log.Fatalf("Error in listener: %s", err)
	}

	if err := s.Serve(ln); err != nil {
		log.Fatalf("Error in server listening %s", err)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	path := ctx.Path()

	// 1. Validate IP of the request
	if !isAuthorized(ctx.RemoteIP()) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	switch string(path) {
	case "/pim", "/pim/":
		// handler you must implement...
		pim.HandlerProcessMsisdnRequest(ctx)

	case "/egg", "/egg/":
		// test endpoint, for status check
		_, _ = fmt.Fprintf(ctx, `
      ID HASH
----------------------------------
	    /\_/\
	  =( °w° )=
	    ) - (  //
	   (__ __)//
----------------------------------
      All rights reserved
MyGaru Inc`)

	default:
		ctx.Logger().Printf("Unsupported http path requested: %q", path)
		ctx.Error("Unsupported http path", fasthttp.StatusNotFound)
	}
}

func logAllFlags() {
	flag.VisitAll(func(f *flag.Flag) {
		log.Printf("FLAG: --%s=%s", f.Name, f.Value)
	})
}

// utils for checking IP //

func parseAllowedIPs(allowedIPs string, allowedIPsMap map[string]struct{}, allowedNets *[]*net.IPNet) error {
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

func isAuthorized(ip net.IP) bool {
	if len(allowedIPsMap) == 0 && len(allowedNets) == 0 {
		return true
	}

	if checkIP(ip, allowedIPsMap, allowedNets) {
		return true
	}

	return false
}

func checkIP(ip net.IP, allowedIPsMap map[string]struct{}, allowedNets []*net.IPNet) bool {
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
