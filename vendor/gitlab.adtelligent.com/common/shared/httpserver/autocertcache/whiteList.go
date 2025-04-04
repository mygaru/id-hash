package autocertcache

import (
	"encoding/json"
	"flag"
	"fmt"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/util"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	autocertWhiteListAPI        = flag.String("autocertWhiteListAPI", "", "WhiteList API address. Used to retrieve list white domains for cert generation")
	autocertWhiteListAPIAuthKey = flag.String("autocertWhiteListAPIAuthKey", "", "Auth key to access WhiteList API")
	refreshInterval             = flag.Duration("autocertWhiteListRefreshInterval", time.Minute, "Time interval to refresh WhiteList")
)

var whiteListValue atomic.Value

func initWhiteList() {
	wl, err := newWhiteList()
	if err != nil {
		log.Errorf("autocertcache: %s", err)
	}
	whiteListValue.Store(wl)
	if len(*autocertWhiteListAPI) == 0 || len(*autocertWhiteListAPIAuthKey) == 0 {
		return
	}
	go func() {
		for {
			util.SleepTill(*refreshInterval)
			wl, err = newWhiteList()
			if err != nil {
				log.Errorf("autocertcache: error while updating WhiteList: %s", err)
			}
			whiteListValue.Store(wl)
		}
	}()
}

type whiteList map[string]struct{}

func (wl *whiteList) check(host string) bool {
	if len(*wl) == 0 {
		return true
	}
	for {
		if _, ok := (*wl)[host]; ok {
			return true
		}
		n := strings.IndexByte(host, '.')
		if n < 0 {
			break
		}
		host = host[n+1:]
	}
	return false
}

var wlClient = &fasthttp.Client{
	ReadTimeout:  3 * time.Second,
	WriteTimeout: 3 * time.Second,
}

func newWhiteList() (whiteList, error) {
	wl := make(whiteList)

	if len(*autocertWhiteListAPI) == 0 || len(*autocertWhiteListAPIAuthKey) == 0 {
		return wl, nil
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetHost(*autocertWhiteListAPI)
	req.Header.Add("Authorization", *autocertWhiteListAPIAuthKey)
	req.SetRequestURI("/api/wl-domains-list")

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	log.Infof("requesting WhiteList from: %s", req.URI().String())
	err := wlClient.DoDeadline(req, resp, time.Now().Add(15*time.Second))
	if err != nil {
		return wl, fmt.Errorf("error while requesting WhiteList: %s", err)
	}

	statusCode := resp.StatusCode()
	switch statusCode {
	case fasthttp.StatusOK:
		var hosts []string
		err := json.Unmarshal(resp.Body(), &hosts)
		if err != nil {
			return wl, fmt.Errorf("error while parsing WhiteList response: %s", err)
		}
		for _, h := range hosts {
			host := extractHost(h)
			wl[host] = struct{}{}
		}
		log.Infof("autocertcache: successfully fetched WhiteList - %d items", len(wl))
	default:
		return wl, fmt.Errorf("unexpected status code for WhiteList request: %d", statusCode)
	}
	return wl, nil
}

func extractHost(s string) string {
	if strings.HasPrefix(s, "https://") {
		s = s[8:]
	}
	if strings.HasPrefix(s, "http://") {
		s = s[7:]
	}

	lvl := strings.Split(s, ".")
	if len(lvl) > 2 {
		cutPos := len(lvl[0]) + 1
		s = s[cutPos:]
	}
	return s
}
