package connlog

import (
	"flag"
	"fmt"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/metric"
	"net"
	"time"
)

var writeConnLog = flag.Bool("writeConnLog", false, "Write connection logs")

func NewConn(addr net.Addr, openConnsPerIP int) {
	if *writeConnLog {
		log.Infof("->[x]: addr = %s, openConnsPerIP = %d", addr.String(), openConnsPerIP)
	}
}

type CloseReason uint8

const (
	ErrCloseAny              = CloseReason(0)
	ErrCloseConnWriteError   = CloseReason(1)
	ErrCloseConnReadErrorEOF = CloseReason(2)
	ErrCloseConnNothingRead  = CloseReason(3)
	ErrCloseConnSmallBuffer  = CloseReason(4)
	ErrCloseConnResetClient  = CloseReason(5)
	ErrCloseConnResetServer  = CloseReason(6)
	ErrorCloseRejectCommon   = CloseReason(7)
	RejectPerIPConnLimit     = CloseReason(8)
)

var (
	httpServerConnRejectIPs = metric.NewCounterVec("httpServerConnRejectIPs")
)

func CloseConn(addr net.Addr, reason CloseReason, requests uint64, created time.Time) {
	if *writeConnLog {
		log.Infof("<-[x]: addr = %s, requests = %d, reason = %d, idle = %q", addr.String(), requests, reason, time.Now().Sub(created))
	}

	if reason == RejectPerIPConnLimit {
		httpServerConnRejectIPs.With(fmt.Sprintf("adrr=%q", addr2ip(addr).String())).Inc()
	}
}

func addr2ip(addr net.Addr) net.IP {
	if tcpaddr, ok := addr.(*net.TCPAddr); ok {
		return tcpaddr.IP
	} else if udpaddr, ok := addr.(*net.UDPAddr); ok {
		return udpaddr.IP
	}

	return net.IPv4zero
}
