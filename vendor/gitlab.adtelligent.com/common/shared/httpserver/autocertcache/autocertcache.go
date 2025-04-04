package autocertcache

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"gitlab.adtelligent.com/common/shared/util"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/metric"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	autocertHostsRegexp     = flag.String("autocertHostsRegexp", ".+", "Regexp for supported TLS autocert hosts")
	autocertCacheDir        = flag.String("autocertCacheDir", "autocert-cache", "Directory to put locally cached autocert certificates")
	autocertCacheServer     = flag.String("autocertCacheServer", "", "autocertcacheserver hostname. autocertCacheDir is used if this flag is empty")
	autocertCacheDisableTLS = flag.Bool("autocertCacheDisableTLS", false, "Whether to disable TLS for requests to autocertcacheserver")
	autocertCacheAuthKey    = flag.String("autocertCacheAuthKey", "", "Authorization key for requests to autocertcacheserver")
	autocertDefaultHost     = flag.String("autocertDefaultHost", "", "Certificate for the given host is returned for clients without SNI")
	autocertFallbackCerts   = flag.String("autocertFallbackCerts", "", "comma-separated fallback certifacates. (HOST1:key:crt,HOST2:key:crt)")
)

var fallbackHostCerts = map[string]*tls.Certificate{}
var defaultHostCertsOnce sync.Once

func NewManager() *autocert.Manager {
	hostPolicyRegexp := regexp.MustCompile(*autocertHostsRegexp)
	defaultHostCertsOnce.Do(initDefaultHostCerts)
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(_ context.Context, host string) error {
			if hostPolicyRegexp.MatchString(host) {
				autocertHostPolicySuccess.Inc()
				return nil
			}
			autocertHostPolicyError.Inc()
			return fmt.Errorf("host %q doesn't match autocertHostsRegexp %q", host, *autocertHostsRegexp)
		},
		Client: &acme.Client{
			DirectoryURL: "https://acme.zerossl.com/v2/DV90",
			//signed certificate by command in ./vendor/src/gitlab.adtelligent.com/common/shared/httpserver/autocertcache/cmd/sign-cert/main.go
			Key: ParseEC(`
-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDAq7s3SYLG8ObGf71q6fEX21HSLYa9UCesfXZjxTY44qMdi7NFehYZj
KPrBH+ybQtqgBwYFK4EEACKhZANiAAQglH6R22YLlA29citjLPhR1oKymgR3OzTl
IHVYjI+p6F3Zq2HedHlpp9rbEPohqETiThqf/dAHODjDRFWKGw/G70fXQnD7vJXl
1rKnXTwgpcEDiiTvnYrCxRhXbVgKUws=
-----END EC PRIVATE KEY-----
`),
			HTTPClient: &http.Client{
				Transport: &loggingTransport{
					RoundTripper: http.DefaultTransport,
				},
			},
			RetryBackoff: func(n int, r *http.Request, resp *http.Response) time.Duration {
				if n >= 2 && resp.StatusCode == 429 {
					return 0
				}
				return acmeDefaultBackoff(n, r, resp)
			},
		},
	}
	if len(*autocertCacheServer) > 0 {
		initWhiteList()
		m.Cache = &Client{
			Addr:          *autocertCacheServer,
			DisableTLS:    *autocertCacheDisableTLS,
			AuthKey:       *autocertCacheAuthKey,
			LocalCacheDir: *autocertCacheDir,
			HostsRegexp:   *autocertHostsRegexp,
		}
	} else {
		m.Cache = autocert.DirCache(*autocertCacheDir)
	}
	return m
}

func ParseEC(s string) *ecdsa.PrivateKey {
	b := decodePEM(s)
	k, err := x509.ParseECPrivateKey(b)
	if err != nil {
		panic(err)
	}
	return k
}

func decodePEM(s string) []byte {
	d, _ := pem.Decode([]byte(s))
	if d == nil {
		panic("no block found in ")
	}
	return d.Bytes
}

func initDefaultHostCerts() {
	if len(*autocertFallbackCerts) == 0 {
		return
	}
	for _, cStr := range strings.Split(*autocertFallbackCerts, ",") {
		//HOST : KEY : CRT
		certParts := strings.Split(cStr, ":")

		cert, err := tls.LoadX509KeyPair(certParts[2], certParts[1])
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				log.Errorf("cant parse autocertFallbackCerts for host %s. Err: %s", certParts[0], err)
			} else {
				log.Fatalf("cant parse autocertFallbackCerts for host %s. Err: %s", certParts[0], err)
			}

			continue
		}

		log.Infof("Add fallback cert for host %s", certParts[0])
		fallbackHostCerts[certParts[0]] = &cert
	}
}

func GetCertificateFunc(m *autocert.Manager, logMissingCertificate bool) func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		hostname := clientHello.ServerName
		if len(hostname) == 0 {
			autocertGetCertificateEmptyHostname.Inc()
			if len(*autocertDefaultHost) == 0 {
				// fall back to default cert
				return nil, nil
			}
			clientHello.ServerName = *autocertDefaultHost
			hostname = clientHello.ServerName
		}

		cert, err := m.GetCertificate(clientHello)
		if err != nil {
			autocertGetCertificateError.Inc()
			if strings.Contains(err.Error(), "missing certificate") {
				if logMissingCertificate {
					log.Errorf("%q<->%q: missing certificate for %q",
						clientHello.Conn.LocalAddr(), clientHello.Conn.RemoteAddr(), hostname)
				}
				autocertGetCertificateMissingCert.Inc()
				// fall back to default cert
				err = nil
			} else {
				if *autocertGetCertificateErrorEnabled {
					log.Errorf("%q<->%q: cannot obtain certificate for %q: %s",
						clientHello.Conn.LocalAddr(), clientHello.Conn.RemoteAddr(), hostname, err)
				}
				//strip unexpected symbols, which broke JSON in /metrics/ response
				hostnameB := bytes.ToLower(util.UnsafeStr2Bytes(hostname))
				hostnameLen := len(hostnameB)
				for i := 0; i < hostnameLen; i++ {
					if (hostnameB[i] >= 'a' && hostnameB[i] <= 'z') ||
						(hostnameB[i] >= '0' && hostnameB[i] <= '9') ||
						hostnameB[i] == '.' ||
						hostnameB[i] == '-' {
						//do nothing
					} else {
						hostnameB[i] = '!' //replace unknown symbols with '!'
					}
				}
				if *autocertGetCertificateErrorEnabled {
					autocertGetCertificateErrorReceive.With(fmt.Sprintf(`host="%s"`, hostnameB)).Inc()
				}
			}

			dotIndex := strings.IndexRune(hostname, '.')
			if dotIndex != -1 {
				if cert, ok := fallbackHostCerts[hostname[dotIndex+1:]]; ok {
					autocertGetCertificateFallbackCertUse.With(fmt.Sprintf(`host="%s"`, hostname[dotIndex+1:])).Inc()
					return cert, nil
				}
			}

			return nil, err
		}
		autocertGetCertificateSuccess.Inc()
		return cert, err
	}
}

var (
	autocertGetCertificateErrorEnabled = flag.Bool("autocertGetCertificateErrorEnabled", false, "")
	autocertHostPolicyError            = metric.NewCounter("autocertHostPolicyError")
	autocertHostPolicySuccess          = metric.NewCounter("autocertHostPolicySuccess")

	autocertGetCertificateEmptyHostname   = metric.NewCounter("autocertGetCertificateEmptyHostname")
	autocertGetCertificateMissingCert     = metric.NewCounter("autocertGetCertificateMissingCert")
	autocertGetCertificateSuccess         = metric.NewCounter("autocertGetCertificateSuccess")
	autocertGetCertificateError           = metric.NewCounter("autocertGetCertificateError")
	autocertGetCertificateErrorReceive    = metric.NewCounterVec("autocertGetCertificateErrorReceive")
	autocertGetCertificateFallbackCertUse = metric.NewCounterVec("autocertGetCertificateFallbackCertUse")
)

type loggingTransport struct {
	http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Infof("sending %q request to %q", req.Method, req.URL)
	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		autocertACMEError.Inc()
		log.Errorf("cannot request %q: %s", req.URL, err)
	} else {
		log.Infof("obtained response from %q. statusCode: %d, contentLength: %d", req.URL, resp.StatusCode, resp.ContentLength)
		if resp.StatusCode != http.StatusOK {
			autocertACMEBadStatusCode.Inc()
		} else {
			autocertACMESuccess.Inc()
		}
	}
	return resp, err
}

var (
	autocertACMESuccess       = metric.NewCounter("autocertACMESuccess")
	autocertACMEError         = metric.NewCounter("autocertACMEError")
	autocertACMEBadStatusCode = metric.NewCounter("autocertACMEBadStatusCode")
)
