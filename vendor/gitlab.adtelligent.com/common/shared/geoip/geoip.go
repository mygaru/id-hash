package geoip

import (
	"flag"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"gitlab.adtelligent.com/common/shared/log"
)

var (
	geoipFilePath        = flag.String("geoipFilePath", "./GeoLite2-Country.mmdb", "Path to geoip file")
	geoipRefreshInterval = flag.Duration("geoipRefreshInterval", time.Hour, "Refresh interval for geoip file")
)

type onlyCountry struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

var doMeOnce []func()
var isReady int64

func OnReady(fn func()) {
	if atomic.LoadInt64(&isReady) == 1 {
		fn()
	}
	doMeOnce = append(doMeOnce, fn)
}

func Init() chan struct{} {
	done := make(chan struct{})
	go func() {
		geoipLoad()
		atomic.StoreInt64(&isReady, 1)
		for i := 0; i < len(doMeOnce); i++ {
			doMeOnce[i]()
		}
		doMeOnce = doMeOnce[:0]
		close(done)
	}()

	return done
}

func geoipLoad() {
	geoipOpen()
	connTypeOpen()
	ispOpen()

	if *geoipRefreshInterval <= 0 {
		return
	}

	go func() {
		for {
			time.Sleep(*geoipRefreshInterval)
			geoipOpen()
			connTypeOpen()
			ispOpen()
		}
	}()
}

func geoipOpen() {
	log.Infof("opening geoip file at %q", *geoipFilePath)

	// open geoip file via FromBytes in order to prevent race conditions
	// on geoip file refresh.

	f, err := os.Open(*geoipFilePath)
	if err != nil {
		log.Fatalf("cannot open geoip file at %q: %s", *geoipFilePath, err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("error when reading geoip file at %q: %s", *geoipFilePath, err)
	}

	db, err := maxminddb.FromBytes(data)
	if err != nil {
		log.Fatalf("error when opening geoip at %q: %s", *geoipFilePath, err)
	}
	log.Infof("verifying opened geoip file...")
	if err = db.Verify(); err != nil {
		log.Fatalf("incorrect geoip file at %q: %s", *geoipFilePath, err)
	}
	log.Infof("successfully verified geoip file")

	// There is no need in closing the previous db, since it has been opened
	// via FromBytes.
	geoipdb.Store(db)
}

var geoipdb atomic.Value

func CountryCodeByIP(ip net.IP) string {
	db := geoipdb.Load().(*maxminddb.Reader)
	c := onlyCountryPool.Get().(*onlyCountry)

	// reset country code to ZZ, since db.Lookup doesn't set it
	// if the country code cannot be found.
	c.Country.ISOCode = "ZZ"

	if err := db.Lookup(ip, c); err != nil {
		log.Fatalf("error when looking up ip=%q in geoipdb: %s", ip, err)
	}
	countryCode := c.Country.ISOCode
	onlyCountryPool.Put(c)
	return countryCode
}

var onlyCountryPool = &sync.Pool{
	New: func() interface{} {
		return &onlyCountry{}
	},
}
