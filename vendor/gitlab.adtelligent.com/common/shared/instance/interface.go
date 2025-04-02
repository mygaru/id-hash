package instance

import (
	"flag"
	"gitlab.adtelligent.com/common/shared/log"
	"net"
)

var (
	maxLocalIPsCount = flag.Int("maxLocalIPsCount", 0, "maxLocalIPsCount")
	cloudenv         = flag.Bool("cloudenv", false, "cloudenv")
)
var LocalTCPAddr []*net.TCPAddr

var greyLocalIPRanges = []string{
	"10.0.0.0/8",
	"127.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

var HasIPv6 bool

func ScanInterface() {

	var localNetworks []*net.IPNet
	for _, ip := range greyLocalIPRanges {
		_, ipnet, err := net.ParseCIDR(ip)
		if err != nil {
			log.Panicf("wrong address %q: %s", ip, err)
		}

		localNetworks = append(localNetworks, ipnet)
	}

	log.Infof("Parsing list of IPs")

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Panicf("Error while getting NET interface, err: %s", err)
	}
	for _, iface := range ifaces {

		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Panicf("Error while extracting interface addresses, err: %s", err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || !ip.IsGlobalUnicast() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				HasIPv6 = true
				continue // not an ipv4 address
			}

			tcpAddr, err := net.ResolveTCPAddr("tcp4", ip.String()+":0")
			if err != nil {
				log.Panicf("Error while resolving local IP: %s, err: %s", ip, err)
			}

			isLocal := false
			if !*cloudenv {
				for _, network := range localNetworks {
					if network.Contains(ip) {
						isLocal = true
						break
					}
				}
			}

			if !isLocal {
				log.Infof("IP %d: %s", len(LocalTCPAddr)+1, ip)
				LocalTCPAddr = append(LocalTCPAddr, tcpAddr)
			} else {
				log.Infof("Local IP (skipped): %s", ip)
			}
		}
	}

	localIPsCount := len(LocalTCPAddr)
	if localIPsCount == 0 {
		log.Panicf("List of local IP addresses is empty")
	}

	if *maxLocalIPsCount > 0 && localIPsCount > *maxLocalIPsCount {
		LocalTCPAddr = LocalTCPAddr[:*maxLocalIPsCount]
	}
}
