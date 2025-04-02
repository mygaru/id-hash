package autocertcache

import (
	"context"
	"fmt"
	"github.com/valyala/fasthttp"
	"gitlab.adtelligent.com/common/shared/log"
	"golang.org/x/crypto/acme/autocert"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Client struct {
	// TCP address of the autocertcache server.
	Addr string

	// Whether to disable TLS.
	DisableTLS bool

	// Authorization key.
	AuthKey string

	// Directory for local cache.
	LocalCacheDir string

	// Allowed hosts regexp.
	HostsRegexp string

	c           *fasthttp.HostClient
	localCache  autocert.DirCache
	hostsRegexp *regexp.Regexp

	once sync.Once
}

func (c *Client) init() {
	c.c = &fasthttp.HostClient{
		Addr:  c.Addr,
		IsTLS: !c.DisableTLS,
	}
	c.localCache = autocert.DirCache(c.LocalCacheDir)
	c.hostsRegexp = regexp.MustCompile(c.HostsRegexp)
}

const http01Prefix = "http-01-"
const http01Suffix = "+http-01"

func (c *Client) Get(ctx context.Context, host string) ([]byte, error) {
	c.once.Do(c.init)

	if err := c.isValid(host); err != nil {
		return nil, fmt.Errorf("requested host is invalid: %s", err)
	}
	data, err := c.getRemote(host)
	if err == nil {
		if len(string(c.localCache)) > 0 {
			err := c.localCache.Put(ctx, host, data)
			if err != nil {
				log.Errorf("cannot store certificate %q in local cache %q: %s", host, c.LocalCacheDir, err)
				// return skipped intentionally, since the local cache is just a fallback.
			}
		}
		log.Infof("successfully obtained certificate %q from %q", host, c.Addr)
		return data, nil
	}
	log.Errorf("error when searching for certificate %q on %q: %s", host, c.Addr, err)

	// fall back to local cache
	if len(string(c.localCache)) > 0 {
		log.Errorf("searching for certificate %q in local cache %q", host, c.LocalCacheDir)
		data, err = c.localCache.Get(ctx, host)
		if err != nil {
			err1 := fmt.Errorf("cannot find certificate for host %q in local cache %q: %s", host, c.LocalCacheDir, err)
			log.Errorf("%s", err1)
			if err != autocert.ErrCacheMiss {
				err = err1
			}
		}
	}
	return data, err
}

func (c *Client) isValid(host string) error {
	if host == "acme_account.key" {
		return nil
	}
	if host == "acme_account+key" {
		return nil
	}
	if strings.HasSuffix(host, ".acme.invalid") || strings.HasPrefix(host, http01Prefix) || strings.HasSuffix(host, http01Suffix) {
		return nil
	}
	if !c.hostsRegexp.MatchString(host) {
		return fmt.Errorf("host %q doesn't match %q", host, c.HostsRegexp)
	}
	v := whiteListValue.Load()
	if v == nil {
		return nil
	}
	wl := v.(whiteList)
	if !wl.check(host) {
		return fmt.Errorf("host %q not in the WhiteList", host)
	}
	return nil
}

func (c *Client) getRemote(host string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetHost(c.Addr)
	req.SetRequestURI("/get")
	qa := req.URI().QueryArgs()
	qa.Set("host", host)
	qa.Set("key", c.AuthKey)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if c.c.IsTLS {
		req.URI().SetScheme("https")
	}

	err := c.c.DoDeadline(req, resp, time.Now().Add(20*time.Second))

	if err != nil {
		return nil, err
	}

	statusCode := resp.StatusCode()

	switch statusCode {
	case fasthttp.StatusOK:
		data := resp.Body()
		data = append([]byte(nil), data...)
		return data, nil
	case fasthttp.StatusNotFound:
		return nil, autocert.ErrCacheMiss
	default:
		return nil, fmt.Errorf("unexpected status code for /get request: %d", statusCode)
	}
}

func (c *Client) Put(ctx context.Context, host string, data []byte) error {
	c.once.Do(c.init)

	if len(string(c.localCache)) > 0 {
		err := c.localCache.Put(ctx, host, data)
		if err != nil {
			log.Errorf("error when putting certificate %q to local cache %q: %s", host, c.LocalCacheDir, err)
			// return skipped intentionally, since the certificate
			// must be stored to remote cache below.
		}
	}
	err := c.putRemote(host, data)
	if err != nil {
		err = fmt.Errorf("error when putting certificate %q to %q: %s", host, c.Addr, err)
		log.Errorf("%s", err)
	} else {
		log.Infof("successfully put certificate %q to %q", host, c.Addr)
	}
	return err
}

func (c *Client) putRemote(host string, data []byte) error {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetHost(c.Addr)
	req.SetRequestURI("/put")
	qa := req.URI().QueryArgs()
	qa.Set("host", host)
	qa.Set("key", c.AuthKey)
	req.SetBody(data)
	req.Header.SetMethod("POST")

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if c.c.IsTLS {
		req.URI().SetScheme("https")
	}

	err := c.c.DoDeadline(req, resp, time.Now().Add(time.Second))

	if err != nil {
		return err
	}

	statusCode := resp.StatusCode()

	switch statusCode {
	case fasthttp.StatusOK:
		return nil
	default:
		return fmt.Errorf("unexpected status code for /put request: %d. Body: %s", statusCode, resp.Body())
	}
}

func (c *Client) Delete(ctx context.Context, host string) error {
	c.once.Do(c.init)

	if len(string(c.localCache)) > 0 {
		err := c.localCache.Delete(ctx, host)
		if err != nil {
			log.Errorf("cannot delete certificate %q from local cache %q: %s", host, c.LocalCacheDir, err)
			// return skipped intentionally, since the certificate
			// must be deleted from remote cache below.
		}
	}
	err := c.deleteRemote(host)
	if err != nil {
		err = fmt.Errorf("cannot delete certificate %q from %q: %s", host, c.Addr, err)
		log.Errorf("%s", err)
	} else {
		log.Infof("successfully deleted certificate %q from %q", host, c.Addr)
	}
	return err
}

func (c *Client) deleteRemote(host string) error {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetHost(c.Addr)
	req.SetRequestURI("/delete")
	qa := req.URI().QueryArgs()
	qa.Set("host", host)
	qa.Set("key", c.AuthKey)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if c.c.IsTLS {
		req.URI().SetScheme("https")
	}

	err := c.c.DoDeadline(req, resp, time.Now().Add(time.Second))

	if err != nil {
		return err
	}

	statusCode := resp.StatusCode()

	switch statusCode {
	case fasthttp.StatusOK:
		return nil
	default:
		return fmt.Errorf("unexpected status code for /delete request: %d", statusCode)
	}
}
