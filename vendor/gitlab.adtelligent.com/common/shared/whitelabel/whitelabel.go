package whitelabel

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"gitlab.adtelligent.com/common/shared/gvl"
	"gitlab.adtelligent.com/common/shared/instance"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/tld"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"strings"
)

var (
	whiteLabelsFlag      = flag.String("whiteLabels", "adtelligent.com", "comma-separated list with full WL hosts")
	unifiedPlatformsFlag = flag.String("unifiedPlatforms", "", "comma-separated list with unified platforms")
	defaultHostFlag      = flag.String("defaultWLHost", "", "default WL host for restricted WL hosts")
)

type WhiteLabel struct {
	Host []byte

	Name []byte

	Vendor *gvl.Vendor

	IsFullWL bool

	CustomPrefix string
}

var defaultWL *WhiteLabel

var (
	whiteLabels      map[string]*WhiteLabel
	unifiedPlatforms map[string]*WhiteLabel
)

var unknownVendor = gvl.Vendor{Name: "Unknown"}

var initialized bool

func Init(csvList string) {

	initialized = true

	if len(*whiteLabelsFlag) > 0 {
		csvList += "," + *whiteLabelsFlag
	}

	if 0 == len(csvList) {
		log.Panicf("List of white labels is empty")
	}

	whiteLabels = initList(csvList)
	unifiedPlatforms = initList(*unifiedPlatformsFlag)

	if len(*defaultHostFlag) > 0 {
		if defaultWL = whiteLabels[*defaultHostFlag]; nil == defaultWL {
			log.Panicf("Unknown default WL host %q", *defaultHostFlag)
		}
	}

	log.Infof("White Labels:")
	i := 0
	for key := range whiteLabels {
		wl := whiteLabels[key]
		if wl.IsFullWL {
			i++
			log.Infof("WL[%d]: host = %s, name = %s, GDPR = %d, prefix = %q", i, wl.Host, wl.Name, wl.Vendor.Id, wl.CustomPrefix)
		}
	}

	log.Infof("Unified Platforms:")
	i = 0
	for key := range unifiedPlatforms {
		wl := unifiedPlatforms[key]
		if nil == wl.Vendor {
			log.Infof("UP[%d]: host = %s, name = %s, GDPR = unknown", i, wl.Host, wl.Name)
		} else {
			log.Infof("UP[%d]: host = %s, name = %s, GDPR = %d", i, wl.Host, wl.Name, wl.Vendor.Id)

		}
	}
}

func GetUnifiedPlatforms(host []byte) *WhiteLabel {
	for key := range unifiedPlatforms {
		wl := unifiedPlatforms[key]
		if bytes.HasSuffix(host, wl.Host) {
			return wl
		}
	}
	return nil
}

func Get(host []byte) *WhiteLabel {
	for key := range whiteLabels {
		wl := whiteLabels[key]
		if bytes.HasSuffix(host, wl.Host) {
			return wl
		}
	}
	return nil
}

func SetServerName(ctx *fasthttp.RequestCtx) {
	if !initialized {
		return
	}
	if wl := Get(ctx.Host()); nil != wl {
		ctx.Response.Header.SetServerBytes(wl.Name)
	}
}

func IsWhiteLabel(host []byte) bool {
	if wl := Get(host); nil != wl {
		return true
	}

	return false
}

// Parse returns WL Host / WL server
// Example: s.adtelligent.com -> adtelligent.com, ads12.adtelligent.com
func Parse(dst []byte, host []byte) (wlHost []byte, wlServer []byte) {
	return ParseRestricted(dst, host, true)
}

func ParseRestricted(dst []byte, host []byte, restricted bool) (wlHost []byte, wlServer []byte) {

	if index := bytes.IndexByte(host, ':'); index > -1 {
		host = host[:index]
	}

	if index := bytes.IndexByte(host, '.'); index > -1 {
		host = host[index:]
	}

	wl := Get(host)
	if nil == wl || (restricted && !wl.IsFullWL) {
		wl = defaultWL
	}

	if 0 == len(wl.CustomPrefix) {
		dst = append(dst, instance.NumberString...)
	} else {
		dst = append(dst, wl.CustomPrefix...)
	}
	dst = append(dst, "."...)
	dst = append(dst, wl.Host...)

	return wl.Host, dst
}

func initList(csvList string) map[string]*WhiteLabel {
	data := make(map[string]*WhiteLabel)

	for _, line := range strings.Split(csvList, ",") {
		line = strings.TrimSpace(line)
		if 0 == len(line) {
			continue
		}
		bb := strings.Split(line, ":")

		wl := &WhiteLabel{
			Host:     []byte(strings.TrimSpace(bb[0])),
			IsFullWL: true,
		}

		if 0 == len(wl.Host) {
			continue
		}

		if u, err := tld.Parse(fmt.Sprintf("https://%s", wl.Host)); nil != err {
			log.Panicf("Broken WL host, host = %s, err = %v", wl.Host, err)
		} else {
			wl.Name = []byte(fmt.Sprintf("%s", cases.Title(language.Und).String(u.Domain)))
		}

		if len(bb) >= 2 && len(bb[1]) >= 0 {
			if vendorId, err := fasthttp.ParseUint([]byte(bb[1])); nil != err {
				log.Panicf("Broken WL host, host = %s, err = %v", wl.Host, err)
			} else if vendorId > 0 {
				if wl.Vendor = gvl.Vendors().GetVendor(uint32(vendorId)); nil == wl.Vendor {
					log.Panicf("Broken WL host, err = unknown vendor %d", vendorId)
				}
			}
		}

		if len(bb) >= 3 && bb[2] == "0" {
			wl.IsFullWL = false
		}

		if len(bb) >= 4 && len(bb[3]) >= 0 {
			wl.CustomPrefix = fmt.Sprintf(`%s%d`, bb[3], *instance.NumberNumeric)
		}

		if nil == wl.Vendor {
			wl.Vendor = &unknownVendor
		}

		if nil == defaultWL && len(*defaultHostFlag) == 0 {
			defaultWL = wl
		}
		data[string(wl.Host)] = wl
	}

	return data

}
