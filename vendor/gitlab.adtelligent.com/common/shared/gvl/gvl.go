package gvl

import (
	"bytes"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"gitlab.adtelligent.com/common/shared/tld"
	"gitlab.adtelligent.com/common/shared/util"
	"sync/atomic"
)

type GVL struct {
	*VendorList

	domains map[string][]*Vendor
}

func Load() (*GVL, error) {
	sc, body, err := fasthttp.Get(nil, "https://vendor-list.consensu.org/v3/vendor-list.json")
	if err != nil {
		return nil, err
	}

	if sc != 200 {
		return nil, fmt.Errorf("not 200 OK, got: %d", sc)
	}

	var p fastjson.Parser
	json, err := p.ParseBytes(body)
	if err != nil {
		return nil, fmt.Errorf("parsing error %v", err)
	}

	vendors := json.GetObject("vendors")

	gvl := &GVL{VendorList: &VendorList{}}
	vendors.Visit(func(key []byte, v *fastjson.Value) {
		vendor := &Vendor{
			Id:   uint32(v.GetUint("id")),
			Name: string(v.GetStringBytes("name")),
		}

		domainsMap := make(map[string]*Vendor_Domain)
		urls := v.GetArray("urls")
		for i := 0; i < len(urls); i++ {
			if domain := parseHost(urls[i].GetStringBytes("privacy")); len(domain) > 0 {
				domainsMap[string(domain)] = &Vendor_Domain{
					Domain: string(domain),
					Loc:    Vendor_Domain_PrivacyList,
				}

				wildcard := "*." + string(domain)
				domainsMap[wildcard] = &Vendor_Domain{
					Domain: wildcard,
					Loc:    Vendor_Domain_PrivacyList,
				}
			}
		}

		if domain := parseHost(v.GetStringBytes("deviceStorageDisclosureUrl")); len(domain) > 0 {
			domainsMap[string(domain)] = &Vendor_Domain{
				Domain: string(domain),
				Loc:    Vendor_Domain_PrivacyList,
			}

			wildcard := "*." + string(domain)
			domainsMap[wildcard] = &Vendor_Domain{
				Domain: wildcard,
				Loc:    Vendor_Domain_DeviceStorageList,
			}
		}

		for idx := range domainsMap {
			domain := domainsMap[idx]
			vendor.Domains = append(vendor.Domains, domain)
		}

		gvl.Vendors = append(gvl.Vendors, vendor)

	})
	gvl.init()

	return gvl, nil
}

func (x *GVL) init() {
	x.domains = make(map[string][]*Vendor)
	for idx := range x.Vendors {
		vendor := x.Vendors[idx]

		for i := 0; i < len(vendor.Domains); i++ {
			domain := vendor.Domains[i]
			if 0 == len(domain.Domain) {
				continue
			}
			if _, ok := x.domains[domain.Domain]; !ok {
				x.domains[domain.Domain] = make([]*Vendor, 0)
			}
			x.domains[domain.Domain] = append(x.domains[domain.Domain], vendor)
		}
	}
}

func (x *GVL) GetVendor(vendorId uint32) *Vendor {
	for idx := range x.Vendors {
		if x.Vendors[idx].Id == vendorId {
			return x.Vendors[idx]
		}
	}

	return nil
}

func (x *GVL) GetVendorByHost(host []byte) (*Vendor, error) {
	// PrivacyList has a priority
	if vendors, ok := x.domains[util.UnsafeBytes2Str(host)]; ok {
		if len(vendors) > 1 {
			return nil, fmt.Errorf("ambiguous identification, host = %q, len = %d", host, len(vendors))
		}

		return vendors[0], nil
	}

	for bb, vendors := range x.domains {
		domain := util.UnsafeStr2Bytes(bb)
		if domain[0] == '*' && bytes.HasSuffix(host, domain[1:]) {
			if len(vendors) > 1 {
				return nil, fmt.Errorf("ambiguous identification, host = %q, len = %d", host, len(vendors))
			}

			return vendors[0], nil
		}
	}

	return nil, nil
}

var vendors atomic.Value

func Vendors() *GVL {
	return vendors.Load().(*GVL)
}
func Init(gvl *GVL) {
	gvl.init()
	vendors.Store(gvl)
}

func GetVendor(host []byte) (*Vendor, error) {
	return Vendors().GetVendorByHost(host)
}

func parseHost(url []byte) []byte {
	//host, _ := util.ParseHost(url)
	//if bytes.Count(host, []byte(".")) >= 3 {
	//	log.Infof("host = %q, c = %d", host, bytes.Count(host, []byte(".")))
	//}

	u, err := tld.Parse(string(url))
	if nil != err {
		host, _ := util.ParseHost(url)
		return host
	}

	//log.Infof("%q = %q (%q)", host, fmt.Sprintf("%s.%s", u.Domain, u.TLD), u.Subdomain)

	return []byte(fmt.Sprintf("%s.%s", u.Domain, u.TLD))
}
