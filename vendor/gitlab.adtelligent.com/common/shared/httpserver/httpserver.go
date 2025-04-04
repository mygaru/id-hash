package httpserver

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	utls "github.com/refraction-networking/utls"
	"github.com/vharitonsky/iniflags"
	"gitlab.adtelligent.com/common/shared/httpauth"
	"gitlab.adtelligent.com/common/shared/httpserver/connlog"
	"gitlab.adtelligent.com/common/shared/whitelabel"
	"golang.org/x/crypto/acme"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/tcplisten"
	"golang.org/x/crypto/acme/autocert"

	"gitlab.adtelligent.com/common/shared/httpserver/autocertcache"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/metric"
	"gitlab.adtelligent.com/common/shared/util"
)

var (
	httpServerAllowedRemoteIPs = flag.String("httpServerAllowedRemoteIPs", "", "NumericList of allowed remote ips. "+
		"If non-empty, the server refuses connections from ips other than listed here")
	httpServerListenBacklog = flag.Int("httpServerListenBacklog", 0, "Backlog value to pass to listen(2). Use system-wide value if no set")
	httpServerCompressLevel = flag.Int("httpServerCompressLevel", 0,
		"Compression level to use for http response compression. 0 means 'no compression'")
	httpServerConcurrency      = flag.Int("httpServerConcurrency", 256*1024, "The maximum number of concurrent requests to the server")
	httpServerDeferAccept      = flag.Bool("httpServerDeferAccept", false, "Whether to use TCP_DEFER_ACCEPT on the listening socket")
	httpServerDisableKeepalive = flag.Bool("httpServerDisableKeepalive", false, "Whether to disable keep-alive connections to the server")
	httpServerFastOpen         = flag.Bool("httpServerFastOpen", true, "Whether to use TCP_FASTOPEN on the listening socket")
	httpServerGetOnly          = flag.Bool("httpServerGetOnly", true, "Whether to accept only GET requests")
	httpServerKeepIdleTimeout  = flag.Duration("httpServerKeepIdleTimeout", time.Minute*2,
		"IDLE period for client connections for http server")
	httpServerKeepAlivePeriod = flag.Duration("httpServerKeepAlivePeriod", 0,
		"Keepalive period for client connections for http server. Zero means 'use default settings'")
	httpServerListenAddr = flag.String("httpServerListenAddr", "",
		"Comma-separated list of TCP addresses to listen to for http requests")
	httpServerListenMultiplier = flag.Int("httpServerListenMultiplier", 1,
		"How many goroutines should listen for incoming connections per each listen addr")
	httpServerListenTLSAddr = flag.String("httpServerListenTLSAddr", "",
		"Comma-separated list of TCP addresses to listen to for https (SSL, TLS) requests")
	httpServerListenUnixAddr     = flag.String("httpServerListenUnixAddr", "", "Comma-separated list of Unix addresses to listen to for http requests")
	httpServerMaxConnsPerIP      = flag.Int("httpServerMaxConnsPerIP", 0, "Maximum number of concurrent connections per client IP. Zero means 'unlimited number of connections per IP allowed'")
	httpServerMaxRequestsPerConn = flag.Int("httpServerMaxRequestsPerConn", 0, "Maxmimum number of requests per connection. Zero means unlimited number of requests")
	httpServerMetricsPassword    = flag.String("httpServerMetricsPassword", "", "Password for /metrics/ page")
	httpServerName               = flag.String("httpServerName", "", "Server name used in http responses")
	httpServerOSReadBufferSize   = flag.Int("httpServerOSReadBufferSize", 0,
		"Per-connection OS read buffer size for http server. Zero means 'use default settings'")
	httpServerOSWriteBufferSize = flag.Int("httpServerOSWriteBufferSize", 0,
		"Per-connection OS write buffer size for http server. Zero means 'use default settings'")
	httpServerReadBufferSize = flag.Int("httpServerReadBufferSize", 0,
		"Per-connection read buffer size for http server. Zero means 'use default settings'")
	httpServerReadTimeout = flag.Duration("httpServerReadTimeout", 3*time.Second,
		"Read timeout for incoming connections to http server. 0 disables read timeout")
	httpServerReduceMemoryUsage = flag.Bool("httpServerReduceMemoryUsage", false, "Whether to reduce httpserver memory usage at the cost of higher CPU usage")
	httpServerReusePort         = flag.Bool("httpServerReusePort", false, "Whether to use SO_REUSEPORT for TCP listen addresses")
	httpServerSessionTicketKey  = flag.String("httpServerSessionTicketKey", "", "Session ticket key for TLS session resumption. "+
		"See https://blog.cloudflare.com/tls-session-resumption-full-speed-and-secure/ for details. "+
		"The key is randomly generated if left blank")
	httpServerSkipFavicon = flag.Bool("httpServerSkipFavicon", true, "Whether to skip /favicon.ico requests")
	//httpServerStatusPassword    = flag.String("httpServerStatusPassword", "foobar", "Password for /status page")
	httpServerTLSAutocertEnable = flag.Bool("httpServerTLSAutocertEnable", false, "Whether to enable TLS autocert. Use httpServerTLS*File if TLS autocert is disabled")
	httpServerTLSCertFile       = flag.String("httpServerTLSCertFile", "", "Comma-separated paths to TLS certificates for httpServerListenTLSAddr addresses. SNI is enabled if multiple certificates are provided. See http://tools.ietf.org/html/rfc4366#section-3.1")
	httpServerTLSKeyFile        = flag.String("httpServerTLSKeyFile", "", "Comma-separated paths to TLS key files for httpServerListenTLSAddr addresses. SNI is enabled if multiple certificates are provided. See http://tools.ietf.org/html/rfc4366#section-3.1")
	httpServerWriteBufferSize   = flag.Int("httpServerWriteBufferSize", 0,
		"Per-connection write buffer size for http server. Zero means 'use default settings'")
	httpServerWriteTimeout         = flag.Duration("httpServerWriteTimeout", 0, "Timeout for response writing. 0 disables timeout")
	httpServerUncompressRequests   = flag.Bool("httpServerUncompressRequests", false, "Whether to uncompress compressed request bodies. May be used for reducing incoming bandwidth for RTB server")
	httpServerUseStdListener       = flag.Bool("httpServerUseStdListener", false, "Use standard listener. This disables httpServerReusePort, httpServerFastOpen and httpServerDeferAccept")
	httpServerMaxRequestBodySize   = flag.Int("httpServerMaxRequestBodySize", fasthttp.DefaultMaxRequestBodySize, "Max request body size")
	httpServerHandleRobotsTXT      = flag.Bool("httpServerHandleRobotsTXT", true, "Is Library should handle robots.txt requests")
	httpServerDisableRobotIndexing = flag.Bool("httpServerDisableRobotIndexing", false, "If true header added X-Robots-Tag:noindex ")
	httpServerTLSMinVersion        = flag.String("httpServerTLSMinVersion", "", "Min version of TLS. Null means use system")
	httpServerTLSMaxVersion        = flag.String("httpServerTLSMaxVersion", "", "Max version of TLS. Null means use system")
	httpServerIPsBlockList         = flag.String("httpServerIPsBlockList", "", "IPs block list")
	httpServerCloseOnShutdown      = flag.Bool("httpServerCloseOnShutdown", false, "Close all connections on shutdown")

	httpServerUTLSInsteadOfTLS           = flag.Bool("httpServerUTLSInsteadOfTLS", false, "Use customized fork of utls lib")
	httpServerUTLSClientHelloExtToStore  = flag.String("httpServerUTLSClientHelloExtToStore", "", "List of TLS ext ids to store in connection")
	httpServerUTLSClientHelloExtToDelete = flag.String("httpServerUTLSClientHelloExtToDelete", "", "List of TLS ext ids to delete on unmarshalling")

	//httpServerRejectGEOMismatch = flag.Bool("httpServerRejectGEOMismatch", false, "If true system rejects connections from different GEO")
)

var servers []*fasthttp.Server
var serversMx sync.Mutex

type StatusHandler func(io.Writer)

func getTcpAddrNetwork(addr string) (network string, err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}

	network = "tcp6"
	if tcpAddr.IP.To4() != nil {
		network = "tcp4"
	}

	return
}

var baseHandler fasthttp.RequestHandler

func Loop(ctx *fasthttp.RequestCtx) {
	httpServerLoop.Inc()
	baseHandler(ctx)
}

func Run(handler fasthttp.RequestHandler, statusHandler StatusHandler) {
	initAllowedIPs()
	httpauth.InitAllowedIPs()

	baseHandler = handler

	initBlockedIPs()
	iniflags.OnFlagChange("httpServerIPsBlockList", initBlockedIPs)

	if *httpServerListenMultiplier > 1 {
		*httpServerReusePort = true
	} else {
		*httpServerListenMultiplier = 1
	}
	addrs := splitAddrs(*httpServerListenAddr, *httpServerListenMultiplier)
	tlsAddrs := splitAddrs(*httpServerListenTLSAddr, *httpServerListenMultiplier)
	unixAddrs := splitAddrs(*httpServerListenUnixAddr, 1)

	totalAddrsCount := len(addrs) + len(tlsAddrs) + len(unixAddrs)
	if totalAddrsCount <= 0 {
		log.Fatalf("At least one httpServerListen*Addr must be set")
	}
	concurrency := (*httpServerConcurrency + totalAddrsCount - 1) / totalAddrsCount

	var wg sync.WaitGroup

	for idx, la := range addrs {
		network, err := getTcpAddrNetwork(la)
		if err != nil {
			log.Fatalf("Can not resolve tcp addr '%s'. Err: %s", la, err)
		}

		wg.Add(1)
		go runServer(network, la, handler, statusHandler, concurrency, nil, false, false, idx, &wg)
	}

	if len(tlsAddrs) > 0 {
		certs, err := loadCertificates()
		if err != nil {
			log.Fatalf("cannot load certificates: %s", err)
		}
		certManager = autocertcache.NewManager()
		if *httpServerTLSAutocertEnable {
			// caused by https://community.letsencrypt.org/t/2018-01-11-update-regarding-acme-tls-sni-and-shared-hosting-infrastructure/50188
			h := certManager.HTTPHandler(nil)
			autocertHandler = fasthttpadaptor.NewFastHTTPHandler(h)
			for idx, la := range tlsAddrs {
				network, err := getTcpAddrNetwork(la)
				if err != nil {
					log.Fatalf("Can not resolve tcp addr '%s'. Err: %s", la, err)
				}

				wg.Add(1)
				go runServer(network, la, handler, statusHandler, concurrency, certs, true, true, idx, &wg)
			}
		} else {
			for idx, la := range tlsAddrs {
				network, err := getTcpAddrNetwork(la)
				if err != nil {
					log.Fatalf("Can not resolve tcp addr '%s'. Err: %s", la, err)
				}

				startCertsCheck(certs, 24*time.Hour)
				wg.Add(1)
				go runServer(network, la, handler, statusHandler, concurrency, certs, true, false, idx, &wg)
			}
		}
	}

	for idx, la := range unixAddrs {
		if err := os.Remove(la); err != nil && !os.IsNotExist(err) {
			log.Fatalf("Unexpected error when trying to remove unix socket file %q: %s", la, err)
		}
		wg.Add(1)
		go runServer("unix", la, handler, statusHandler, concurrency, nil, false, false, idx, &wg)
	}

	wg.Wait()
}

func Shutdown() {
	serversMx.Lock()
	defer serversMx.Unlock()

	for i := len(servers) - 1; i >= 0; i-- {
		if err := servers[i].Shutdown(); err != nil {
			log.Errorf("Error shutting down http server: %s", err)
		}
		servers[i] = nil
		servers = servers[:i]
	}
}

var certsStore atomic.Value

func startCertsCheck(certs []tls.Certificate, refreshInterval time.Duration) {
	certsStore.Store(certs)
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	go func() {
		for {
			<-ticker.C
			oldCerts := certsStore.Load().([]tls.Certificate)
			for i := range oldCerts {
				if isCertExpired(oldCerts[i]) {
					newCerts, err := loadCertificates()
					if err != nil {
						log.Errorf("cannot load certificates: %s", err)
						break
					}
					certsStore.Store(newCerts)
					break
				}
			}
		}
	}()
}

func isCertExpired(cert tls.Certificate) bool {
	leaf, err := leaf(cert)
	if err != nil {
		return true
	}
	if time.Now().Add(72 * time.Hour).After(leaf.NotAfter) {
		return true
	}
	return false
}

func leaf(cert tls.Certificate) (*x509.Certificate, error) {
	if cert.Leaf != nil {
		return cert.Leaf, nil
	}
	return x509.ParseCertificate(cert.Certificate[0])
}

var (
	certManager     *autocert.Manager
	autocertHandler fasthttp.RequestHandler
)

func loadCertificates() ([]tls.Certificate, error) {
	if len(*httpServerTLSKeyFile) == 0 && !*httpServerTLSAutocertEnable {
		return nil, fmt.Errorf("httpServerTLSKeyFile must be set if httpServerListenTLSAddr is set")
	}
	if len(*httpServerTLSCertFile) == 0 && !*httpServerTLSAutocertEnable {
		return nil, fmt.Errorf("httpServerTLSCertFile must be set if httpServerListenTLSAddr is set")

	}
	if len(*httpServerTLSKeyFile) == 0 || len(*httpServerTLSCertFile) == 0 {
		// autocert works perfectly without tls certs
		return nil, nil
	}

	tlsKeyFiles := strings.Split(*httpServerTLSKeyFile, ",")
	tlsCertFiles := strings.Split(*httpServerTLSCertFile, ",")
	if len(tlsKeyFiles) != len(tlsCertFiles) {
		return nil, fmt.Errorf("httpServerTLSKeyFile=%q and httpServerTLSCertFile=%q must have the identical number of items",
			*httpServerTLSKeyFile, *httpServerTLSCertFile)
	}

	var certs []tls.Certificate
	for i := 0; i < len(tlsCertFiles); i++ {
		tlsCertFile := tlsCertFiles[i]
		tlsKeyFile := tlsKeyFiles[i]
		cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS key pair from certFile=%q and keyFile=%q: %s",
				tlsCertFile, tlsKeyFile, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func splitAddrs(addrs string, multiplier int) []string {
	if len(addrs) == 0 {
		return nil
	}

	addrsList := strings.Split(addrs, ",")
	return applyMultiplier(addrsList, multiplier)
}

func applyMultiplier(ss []string, multiplier int) []string {
	var outSS []string
	for i := 0; i < multiplier; i++ {
		outSS = append(outSS, ss...)
	}
	return outSS
}

func runServer(network string, la string, handler fasthttp.RequestHandler, statusHandler StatusHandler, concurrency int,
	certs []tls.Certificate, isTLS, isTLSAutocert bool, idx int, wg *sync.WaitGroup) {
	defer wg.Done()

	s := &fasthttp.Server{
		Handler:               getHTTPHandler(handler, statusHandler),
		Name:                  *httpServerName,
		DisableKeepalive:      *httpServerDisableKeepalive,
		ReadBufferSize:        *httpServerReadBufferSize,
		WriteBufferSize:       *httpServerWriteBufferSize,
		ReadTimeout:           *httpServerReadTimeout,
		WriteTimeout:          *httpServerWriteTimeout,
		MaxConnsPerIP:         *httpServerMaxConnsPerIP,
		MaxRequestsPerConn:    *httpServerMaxRequestsPerConn,
		Concurrency:           concurrency,
		ReduceMemoryUsage:     *httpServerReduceMemoryUsage,
		GetOnly:               *httpServerGetOnly,
		Logger:                log.FasthttpErrorLogger,
		MaxRequestBodySize:    *httpServerMaxRequestBodySize,
		TCPKeepalivePeriod:    *httpServerKeepAlivePeriod,
		IdleTimeout:           *httpServerKeepIdleTimeout,
		MaxIdleWorkerDuration: *httpServerKeepIdleTimeout,
		CloseOnShutdown:       *httpServerCloseOnShutdown,
	}

	serversMx.Lock()
	servers = append(servers, s)
	serversMx.Unlock()

	s.TCPKeepalive = *httpServerKeepAlivePeriod > 0

	var ln net.Listener
	var err error

	if strings.HasPrefix(network, "tcp") && !*httpServerUseStdListener {
		cfg := &tcplisten.Config{
			DeferAccept: *httpServerDeferAccept,
			FastOpen:    *httpServerFastOpen,
			ReusePort:   *httpServerReusePort,
			Backlog:     *httpServerListenBacklog,
		}
		ln, err = cfg.NewListener(network, la)
	} else {
		ln, err = net.Listen(network, la)
	}
	if err != nil {
		log.Fatalf("cannot listen %q: %s", la, err)
	}

	if network == "unix" {
		if err = os.Chmod(la, 0777); err != nil {
			log.Fatalf("cannot set chmod 0777 to %q: %s", la, err)
		}
	}
	ln = &customListener{
		Listener: ln,
		isTLS:    isTLS,
		network:  network,
	}

	label := network

	if isTLS {
		label += "TLS"
		ln = getSecureListener(ln, isTLSAutocert, certs)
	}

	s.ConnState = func(conn net.Conn, state fasthttp.ConnState, openConnsPerIP int) {
		switch state {
		case fasthttp.StateNew:
			httpServerConnNew.Inc()
			connlog.NewConn(conn.RemoteAddr(), openConnsPerIP)
		case fasthttp.StateActive:
			httpServerConnActive.Inc()
		case fasthttp.StateIdle:
			httpServerConnIdle.Inc()
		case fasthttp.StateHijacked:
			httpServerConnHijacked.Inc()
		}
	}

	labelCopy := []byte(label)
	labelCopy[0] = byte(unicode.ToUpper(rune(labelCopy[0])))

	labelName := fmt.Sprintf("httpServer%sOpenConnections", labelCopy)
	if idx > 0 {
		labelName += fmt.Sprintf("%d", idx)
	}

	metric.NewGauge(labelName, func() uint64 {
		return uint64(s.GetOpenConnectionsCount())
	})

	s.ConnIDLE = func(conn net.Conn, id, requests uint64, idle, read, write time.Duration) {
		if requests == 1 {
			httpServerOpenDuration.Update(read.Seconds())
		}
		httpServerIdleDuration.Update(idle.Seconds())
		httpServerReadDuration.Update(read.Seconds())
		httpServerWriteDuration.Update(write.Seconds())
	}

	s.ConnReject = func(addr net.Addr, err error) {
		if errors.Is(err, fasthttp.ErrPerIPConnLimit) {
			httpServerConnRejectIPLimit.Inc()
			connlog.CloseConn(addr, connlog.RejectPerIPConnLimit, 0, time.Now())
		} else {
			httpServerConnReject.Inc()
			connlog.CloseConn(addr, connlog.ErrorCloseRejectCommon, 0, time.Now())
		}
	}

	s.ConnClose = func(conn net.Conn, id, requests uint64, created time.Time, reason error) {
		isnew := requests == 1
		if reason == nil {
			httpServerConnCloseAny.Inc()
			connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseAny, requests, created)
		} else {
			if _, ok := reason.(fasthttp.ConnWriteError); ok {
				httpServerConnWriteError.Inc()
				connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnWriteError, requests, created)
			} else if errRead, ok := reason.(fasthttp.ConnReadError); ok {
				err := errRead.NestedError()
				if err == io.EOF {
					connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnReadErrorEOF, requests, created)
					if isnew {
						httpServerConnFirstReadEOF.Inc()
					} else {
						httpServerConnSecondReadEOF.Inc()
					}

				} else if _, ok := err.(fasthttp.ErrNothingRead); ok {
					connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnNothingRead, requests, created)
					if isnew {
						httpServerConnFirstReadNothing.Inc()
					} else {
						httpServerConnSecondReadNothing.Inc()
					}
				} else if _, ok := err.(fasthttp.ErrSmallBuffer); ok {
					connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnSmallBuffer, requests, created)
					httpServerConnReadSmallBuffer.Inc()
				} else {
					httpServerConnReadOther.Inc()
				}
			} else if reason == fasthttp.ConnResetClient {
				connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnResetClient, requests, created)
				httpServerConnResetClient.Inc()
			} else if reason == fasthttp.ConnResetServer {
				connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseConnResetServer, requests, created)
				httpServerConnResetServer.Inc()
			} else {
				connlog.CloseConn(conn.RemoteAddr(), connlog.ErrCloseAny, requests, created)
				httpServerConnCloseAny.Inc()
			}
		}

		httpServerConnDuration.UpdateDuration(created)
		httpServerReqs2Conn.Update(float64(requests))
	}

	if err := s.Serve(ln); err != nil {
		log.Fatalf("error in server listening for %q: %s", la, err)
	}
}

func getSecureListener(inner net.Listener, isTLSAutocert bool, certs []tls.Certificate) (ln net.Listener) {
	if !*httpServerUTLSInsteadOfTLS {
		return getTLSListener(inner, isTLSAutocert, certs)
	}

	var utlsCerts []utls.Certificate
	for _, cert := range certs {
		utlsCert := utls.Certificate{
			Certificate:                 cert.Certificate,
			PrivateKey:                  cert.PrivateKey,
			OCSPStaple:                  cert.OCSPStaple,
			SignedCertificateTimestamps: cert.SignedCertificateTimestamps,
			Leaf:                        cert.Leaf,
		}

		for _, ssa := range cert.SupportedSignatureAlgorithms {
			utlsCert.SupportedSignatureAlgorithms = append(utlsCert.SupportedSignatureAlgorithms, utls.SignatureScheme(ssa))
		}

		utlsCerts = append(utlsCerts, utlsCert)
	}

	return getUTLSListener(inner, isTLSAutocert, utlsCerts)
}

func getUTLSListener(inner net.Listener, isTLSAutocert bool, certs []utls.Certificate) (ln net.Listener) {
	tlsConfig := &utls.Config{
		// See https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
		PreferServerCipherSuites: true,
		CurvePreferences: []utls.CurveID{
			utls.CurveP256,
			utls.X25519, // Go 1.8 only
		},
		//for tls-alpn ACME challenges
		NextProtos: []string{
			"http/1.1",
			acme.ALPNProto, // enable tls-alpn ACME challenges
		},
	}

	if len(*httpServerUTLSClientHelloExtToStore) > 0 {
		tlsConfig.ClientHelloCustomExtToStore = make(map[uint16]struct{})
	}
	if len(*httpServerUTLSClientHelloExtToDelete) > 0 {
		tlsConfig.ClientHelloCustomExtToDelete = make(map[uint16]struct{})
	}

	for _, strExtId := range strings.Split(*httpServerUTLSClientHelloExtToStore, ",") {
		strExtId = strings.TrimSpace(strExtId)
		if len(strExtId) == 0 {
			continue
		}

		extId, err := strconv.ParseUint(strExtId, 10, 16)
		if err != nil {
			log.Fatalf("Can not parse httpServerUTLSClientHelloExtToStore config. Value: %s, Err: %s", strExtId, err)
		}

		tlsConfig.ClientHelloCustomExtToStore[uint16(extId)] = struct{}{}
	}

	for _, strExtId := range strings.Split(*httpServerUTLSClientHelloExtToDelete, ",") {
		strExtId = strings.TrimSpace(strExtId)
		if len(strExtId) == 0 {
			continue
		}

		extId, err := strconv.ParseUint(strExtId, 10, 16)
		if err != nil {
			log.Fatalf("Can not parse httpServerUTLSClientHelloExtToDelete config. Value: %s, Err: %s", strExtId, err)
		}

		tlsConfig.ClientHelloCustomExtToDelete[uint16(extId)] = struct{}{}
	}

	if *httpServerTLSMinVersion != "" {
		switch *httpServerTLSMinVersion {
		case "1.0":
			tlsConfig.MinVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MinVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MinVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MinVersion = tls.VersionTLS13
		}
	}

	if *httpServerTLSMaxVersion != "" {
		switch *httpServerTLSMaxVersion {
		case "1.0":
			tlsConfig.MaxVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MaxVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MaxVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}

	// This is for SNI (see http://tools.ietf.org/html/rfc4366#section-3.1)
	tlsConfig.Certificates = certs
	tlsConfig.BuildNameToCertificate()

	if certManager == nil {
		log.Fatalf("BUG: got nil certManager")
	}

	autocertGetCertificate := autocertcache.GetCertificateFunc(certManager, certs == nil)

	autocertGetCertificateFuncProxy := func(info *utls.ClientHelloInfo) (cert *utls.Certificate, err error) {
		tlsInfo := &tls.ClientHelloInfo{
			CipherSuites:      info.CipherSuites,
			ServerName:        info.ServerName,
			SupportedPoints:   info.SupportedPoints,
			SupportedProtos:   info.SupportedProtos,
			SupportedVersions: info.SupportedVersions,
			Conn:              info.Conn,
		}

		for _, i := range info.SupportedCurves {
			tlsInfo.SupportedCurves = append(tlsInfo.SupportedCurves, tls.CurveID(i))
		}
		for _, i := range info.SignatureSchemes {
			tlsInfo.SignatureSchemes = append(tlsInfo.SignatureSchemes, tls.SignatureScheme(i))
		}

		tlsCert, err := autocertGetCertificate(tlsInfo)
		if err != nil {
			return
		}

		cert = &utls.Certificate{
			Certificate:                 tlsCert.Certificate,
			PrivateKey:                  tlsCert.PrivateKey,
			OCSPStaple:                  tlsCert.OCSPStaple,
			SignedCertificateTimestamps: tlsCert.SignedCertificateTimestamps,
			Leaf:                        tlsCert.Leaf,
		}

		for _, i := range tlsCert.SupportedSignatureAlgorithms {
			cert.SupportedSignatureAlgorithms = append(cert.SupportedSignatureAlgorithms, utls.SignatureScheme(i))
		}

		return
	}

	if isTLSAutocert {
		tlsConfig.GetCertificate = autocertGetCertificateFuncProxy
	} else {
		tlsConfig.GetCertificate = func(clientHello *utls.ClientHelloInfo) (*utls.Certificate, error) {
			// Answer autocert requests.
			host := strings.ToLower(clientHello.ServerName)
			if strings.HasSuffix(host, ".acme.invalid") {
				return autocertGetCertificateFuncProxy(clientHello)
			}
			return nil, nil
		}
	}

	// See https://blog.cloudflare.com/tls-session-resumption-full-speed-and-secure/
	initSessionTicketKey(&tlsConfig.SessionTicketKey)
	ln = utls.NewListener(inner, tlsConfig)
	return
}

func getTLSListener(inner net.Listener, isTLSAutocert bool, certs []tls.Certificate) (ln net.Listener) {
	tlsConfig := &tls.Config{
		// See https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519, // Go 1.8 only
		},
		//for tls-alpn ACME challenges
		NextProtos: []string{
			"http/1.1",
			acme.ALPNProto, // enable tls-alpn ACME challenges
		},
	}

	if *httpServerTLSMinVersion != "" {
		switch *httpServerTLSMinVersion {
		case "1.0":
			tlsConfig.MinVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MinVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MinVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MinVersion = tls.VersionTLS13
		}
	}

	if *httpServerTLSMaxVersion != "" {
		switch *httpServerTLSMaxVersion {
		case "1.0":
			tlsConfig.MaxVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MaxVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MaxVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}

	if certManager == nil {
		log.Fatalf("BUG: got nil certManager")
	}

	autocertGetCertificate := autocertcache.GetCertificateFunc(certManager, certs == nil)
	if isTLSAutocert {
		// This is for SNI (see http://tools.ietf.org/html/rfc4366#section-3.1)
		tlsConfig.Certificates = certs
		tlsConfig.BuildNameToCertificate()
		tlsConfig.GetCertificate = autocertGetCertificate
	} else {
		tlsConfig.GetCertificate = func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			certificates := certsStore.Load().([]tls.Certificate)
			for _, cert := range certificates {
				if err := clientHello.SupportsCertificate(&cert); err == nil {
					return &cert, nil
				}
			}
			// Answer autocert requests.
			host := strings.ToLower(clientHello.ServerName)
			if strings.HasSuffix(host, ".acme.invalid") {
				return autocertGetCertificate(clientHello)
			}
			return nil, nil
		}
	}

	// See https://blog.cloudflare.com/tls-session-resumption-full-speed-and-secure/
	initSessionTicketKey(&tlsConfig.SessionTicketKey)
	ln = tls.NewListener(inner, tlsConfig)

	return
}

var (
	httpServerConnWriteError      = metric.NewCounter("httpServerConnWriteError")
	httpServerConnResetClient     = metric.NewCounter("httpServerConnResetClient")
	httpServerConnResetServer     = metric.NewCounter("httpServerConnResetServer")
	httpServerConnReadSmallBuffer = metric.NewCounter("httpServerConnReadSmallBuffer")
	httpServerConnReadOther       = metric.NewCounter("httpServerConnReadOther")
	httpServerConnCloseAny        = metric.NewCounter("httpServerConnCloseAny")
	httpServerConnAccept          = metric.NewCounter("httpServerConnAccept")
	httpServerConnNew             = metric.NewCounter("httpServerConnNew")
	httpServerConnActive          = metric.NewCounter("httpServerConnActive")
	httpServerConnIdle            = metric.NewCounter("httpServerConnIdle")
	httpServerConnHijacked        = metric.NewCounter("httpServerConnHijacked")
	httpServerConnReject          = metric.NewCounter("httpServerConnReject")
	httpServerConnRejectIPLimit   = metric.NewCounter("httpServerConnRejectIPLimit")
	httpServerReqs2Conn           = metric.NewHistogram("httpServerReqs2Conn")
	httpServerConnDuration        = metric.NewHistogram("httpServerConnDuration")
	httpServerIdleDuration        = metric.NewHistogram("httpServerIdleDuration")
	httpServerOpenDuration        = metric.NewHistogram("httpServerOpenDuration")
	httpServerWriteDuration       = metric.NewHistogram("httpServerWriteDuration")
	httpServerReadDuration        = metric.NewHistogram("httpServerReadDuration")

	httpServerConnReadEOF       = metric.NewCounterVec("httpServerConnReadEOF")
	httpServerConnFirstReadEOF  = httpServerConnReadEOF.With(`req="first"`)
	httpServerConnSecondReadEOF = httpServerConnReadEOF.With(`req="second"`)

	httpServerConnReadNothing       = metric.NewCounterVec("httpServerConnReadNothing")
	httpServerConnFirstReadNothing  = httpServerConnReadNothing.With(`req="first"`)
	httpServerConnSecondReadNothing = httpServerConnReadNothing.With(`req="second"`)
)

func initSessionTicketKey(dst *[32]byte) {
	if len(*httpServerSessionTicketKey) == 0 {
		return
	}
	*dst = sha256.Sum256([]byte(*httpServerSessionTicketKey))
}

type customListener struct {
	net.Listener
	isTLS   bool
	network string
}

func (ln *customListener) Accept() (c net.Conn, err error) {
	httpServerConnAccept.Inc()

	c, err = ln.Listener.Accept()
	if err != nil {
		if c != nil {
			log.Panicf("BUG: accept returned non-nil c=%#v with error %s", c, err)
		}
		return nil, err
	}

	if tcpConn, ok := c.(*net.TCPConn); ok {
		if err = setupTCPConn(tcpConn); err != nil {
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

func setupTCPConn(c *net.TCPConn) error {
	var err error

	if *httpServerOSReadBufferSize > 0 {
		if err = c.SetReadBuffer(*httpServerOSReadBufferSize); err != nil {
			return err
		}
	}
	if *httpServerOSWriteBufferSize > 0 {
		if err = c.SetWriteBuffer(*httpServerOSWriteBufferSize); err != nil {
			return err
		}
	}
	return nil
}

var (
	robotsTXTData = []byte("User-agent: *\nDisallow: /")
	strMetrics    = []byte("/metrics/")
	acmeChallenge = []byte("/.well-known/acme-challenge/")
)

var (
	httpServerResponseBodySize   = metric.NewHistogram("httpServerResponseBodySize")
	httpServerResponseBytes      = metric.NewCounter("httpServerResponseBytes")
	httpServerRequest            = metric.NewCounter("httpServerRequest")
	httpServerRequestBlockedIPs  = metric.NewCounter("httpServerRequestBlockedIPs")
	httpServerRequestUnspecified = metric.NewCounter("httpServerRequestUnspecified")
	httpServerRequestTLS         = metric.NewCounter("httpServerRequestTLS")
	httpServerRequestIPv6        = metric.NewCounter("httpServerRequestIPv6")
	httpServerLoop               = metric.NewCounter("httpServerLoop")
	httpServerRequestDuration    = metric.NewHistogram("httpServerRequestDuration")
	httpServerStatusRequest      = metric.NewCounter("httpServerStatusRequest")
	httpServerPageNotFound       = metric.NewCounter("httpServerPageNotFound")
)

func getHTTPHandler(handler fasthttp.RequestHandler, statusHandler StatusHandler) fasthttp.RequestHandler {
	h := func(ctx *fasthttp.RequestCtx) {
		startTime := time.Now()
		if !isAllowedIP(ctx) {
			_, _ = fmt.Fprintf(ctx, "Request from the ip %q is forbidden", ctx.RemoteIP())
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.Logger().Printf("Request from disallowed ip")
			whitelabel.SetServerName(ctx)
			return
		}

		if isBlockedIP(ctx) {
			httpServerRequestBlockedIPs.Inc()
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			//ctx.Logger().Printf("Request from disallowed ip")
			whitelabel.SetServerName(ctx)
			return
		}

		isLoop := ctx.RemoteIP().IsUnspecified()
		if isLoop {
			httpServerRequestUnspecified.Inc()
		}

		httpServerRequest.Inc()
		if ctx.IsTLS() {
			httpServerRequestTLS.Inc()
		}
		if ctx.RemoteIP().To4() == nil {
			httpServerRequestIPv6.Inc()
		}

		httpHandler(ctx, handler, statusHandler)
		whitelabel.SetServerName(ctx)

		if !ctx.IsBodyStream() {
			bodySize := len(ctx.Response.Body())
			httpServerResponseBodySize.Update(float64(bodySize))

			if !isLoop {
				httpServerResponseBytes.Add(bodySize)
			}
		}
		httpServerRequestDuration.UpdateDuration(startTime)
	}
	if *httpServerCompressLevel > 0 {
		level := *httpServerCompressLevel
		if level > 9 {
			level = 9
		}
		h = fasthttp.CompressHandlerLevel(h, level)
	}
	return h
}

func httpHandler(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, statusHandler StatusHandler) {
	baseHTTPHandler(ctx, handler, statusHandler)
	//for custom handler
	if *httpServerDisableRobotIndexing {
		ctx.Response.Header.Set(fasthttp.HeaderXRobotsTag, "noindex")
	}
}

func baseHTTPHandler(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, statusHandler StatusHandler) {
	path := ctx.Path()
	if bytes.HasPrefix(path, strMetrics) {
		if string(ctx.QueryArgs().Peek("key")) != *httpServerMetricsPassword {
			ctx.Error("Invalid key", fasthttp.StatusForbidden)
			return
		}
		metric.HTTPHandler(ctx, path)
		return
	}

	// caused by https://community.letsencrypt.org/t/2018-01-11-update-regarding-acme-tls-sni-and-shared-hosting-infrastructure/50188
	if bytes.HasPrefix(path, acmeChallenge) {
		if autocertHandler == nil {
			log.Errorf("autocert: acme challenged without autocertHandler")
			return
		}
		log.Infof("challenged with %q", path)
		autocertHandler(ctx)
		return
	}

	switch string(path) {
	case "/favicon.ico":
		if *httpServerSkipFavicon {
			ctx.Error("Not Found", fasthttp.StatusNotFound)
			return
		}
	case "/robots.txt":
		if *httpServerHandleRobotsTXT {
			ctx.Success("text/plain", robotsTXTData)
			return
		}
	case "/status":
		if !httpauth.IsAuthorized(ctx) {
			ctx.Error("Invalid key", fasthttp.StatusForbidden)
			return
		}
		httpServerStatusRequest.Inc()
		util.PrintAllFlags(ctx)
		util.PrintStatus(ctx)
		if statusHandler != nil {
			statusHandler(ctx)
		}
		ctx.SetContentType("text/plain")
		return
	case "/prometheus":
		if string(ctx.QueryArgs().Peek("key")) != *httpServerMetricsPassword {
			ctx.Error("Invalid key", fasthttp.StatusForbidden)
			return
		}
		metric.HTTPPrometheusHandler(ctx)
		return
	}

	if handler == nil {
		httpServerPageNotFound.Inc()
		ctx.Logger().Printf("Page not found: %q", path)
		ctx.Error("Page not found", fasthttp.StatusNotFound)
		return
	}
	if err := tryUncompressRequest(ctx); err != nil {
		ctx.Logger().Printf("Error when uncompressing request body: %s", err)
		return
	}
	handler(ctx)
}

func tryUncompressRequest(ctx *fasthttp.RequestCtx) error {
	if !*httpServerUncompressRequests {
		return nil
	}

	ce := ctx.Request.Header.Peek("Content-Encoding")
	switch string(ce) {
	case "", "identity":
		// nothing to uncompress
		return nil
	case "gzip":
		body, err := ctx.Request.BodyGunzip()
		if err != nil {
			httpServerUncompressGunzipError.Inc()
			ctx.Error("Cannot unzip request body", fasthttp.StatusBadRequest)
			return fmt.Errorf("cannot unzip request body: %s", err)
		}
		ctx.Request.Header.Del("Content-Encoding")
		ctx.Request.SetBody(body)
		httpServerUncompressGunzipSuccess.Inc()
	case "flate":
		body, err := ctx.Request.BodyInflate()
		if err != nil {
			httpServerUncompressInflateError.Inc()
			ctx.Error("Cannot inflate request body", fasthttp.StatusBadRequest)
			return fmt.Errorf("cannot inflate request body: %s", err)
		}
		ctx.Request.Header.Del("Content-Encoding")
		ctx.Request.SetBody(body)
		httpServerUncompressInflateSuccess.Inc()
	default:
		httpServerUncompressUnsupportedContentEncoding.Inc()
		ctx.Error("Unsupported Content-Encoding", fasthttp.StatusBadRequest)
		return fmt.Errorf("unsupported Content-Encoding: %q", ce)
	}
	return nil
}

var (
	httpServerUncompressGunzipSuccess              = metric.NewCounter("httpServerUncompressGunzipSuccess")
	httpServerUncompressGunzipError                = metric.NewCounter("httpServerUncompressGunzipError")
	httpServerUncompressInflateSuccess             = metric.NewCounter("httpServerUncompressInflateSuccess")
	httpServerUncompressInflateError               = metric.NewCounter("httpServerUncompressInflateError")
	httpServerUncompressUnsupportedContentEncoding = metric.NewCounter("httpServerUncompressUnsupportedContentEncoding")
)

var allowedIPsMap = make(map[string]struct{})
var allowedNets = make([]*net.IPNet, 0)

func initAllowedIPs() {
	if len(*httpServerAllowedRemoteIPs) == 0 {
		return
	}

	if err := httpauth.ParseAllowedIPs(*httpServerAllowedRemoteIPs, allowedIPsMap, &allowedNets); err != nil {
		log.Fatalf("Error parsing httpServerAllowedRemoteIPs: %v", err)
	}
}

func isAllowedIP(ctx *fasthttp.RequestCtx) bool {
	if len(allowedIPsMap) == 0 && len(allowedNets) == 0 {
		return true
	}

	return httpauth.CheckIP(ctx.RemoteIP(), allowedIPsMap, allowedNets)
}

var blockedIPs atomic.Value

func initBlockedIPs() {
	blockedIPsList := make(map[string]bool)
	if len(*httpServerIPsBlockList) > 0 {
		for _, bb := range strings.Split(*httpServerIPsBlockList, ",") {
			ipdata := strings.Split(bb, "/")
			ip := net.ParseIP(ipdata[0])
			if nil == ip {
				continue
			}

			pass := len(ipdata) == 2 && ipdata[1] == "1"

			blockedIPsList[ip.String()] = pass
			log.Infof("Blocked IP: %q, pass = %v", ip.String(), pass)
		}
	}
	blockedIPs.Store(blockedIPsList)
}

func isBlockedIP(ctx *fasthttp.RequestCtx) bool {
	blockedIPsMap := blockedIPs.Load().(map[string]bool)
	if len(blockedIPsMap) == 0 {
		return false
	}
	ip := ctx.RemoteIP()
	pass, ok := blockedIPsMap[ip.String()]
	if ok {
		if pass {
			ctx.SetUserValue("trafficFromBlockedIP", true)
			return false
		}
		return true
	}

	bb := ctx.Request.Header.Peek("X-Forwarded-For")
	if len(bb) == 0 {
		bb = ctx.Request.Header.Peek("X-Real-IP")
	}

	if len(bb) > 0 {
		XForwardedIPs := strings.Split(string(bb), ",")
		for i := 0; i < len(XForwardedIPs); i++ {
			if ip = net.ParseIP(strings.TrimSpace(XForwardedIPs[i])); ip != nil {
				pass, ok = blockedIPsMap[ip.String()]
				if ok {
					if pass {
						ctx.SetUserValue("trafficFromBlockedIP", true)
						return false
					}
					return true
				}
			}
		}
	}

	return false
}

func IsTLSEnrichEnabled() bool {
	return *httpServerUTLSInsteadOfTLS
}
