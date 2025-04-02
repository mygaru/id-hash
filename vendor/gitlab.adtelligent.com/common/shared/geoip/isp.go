package geoip

import (
	"flag"
	"github.com/oschwald/maxminddb-golang"
	"gitlab.adtelligent.com/common/shared/log"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

var (
	ispFilePath = flag.String("ispFilePath", "", "Path to maxmind isp database")
)

type isp struct {
	ISP string `maxminddb:"isp"`
}

func ispOpen() {
	if *ispFilePath == "" {
		return
	}

	log.Infof("opening mmdb file at %q", *ispFilePath)

	f, err := os.Open(*ispFilePath)
	if err != nil {
		log.Errorf("cannot open mmdb file at %q: %s", *ispFilePath, err)
		return
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Errorf("error when reading mmdb file at %q: %s", *ispFilePath, err)
		return
	}

	db, err := maxminddb.FromBytes(data)
	if err != nil {
		log.Errorf("error when opening mmdb at %q: %s", *ispFilePath, err)
		return
	}

	log.Infof("verifying opened mmdb file...")
	if err = db.Verify(); err != nil {
		log.Errorf("incorrect mmdb file at %q: %s", *ispFilePath, err)
		return
	}
	log.Infof("successfully verified mmdb file")

	ispDb.Store(db)
}

var ispDb atomic.Value

func ISPByIP(ip net.IP) string {
	db, ok := ispDb.Load().(*maxminddb.Reader)
	if !ok || db == nil {
		return ""
	}

	c := ispPool.Get().(*isp)
	defer ispPool.Put(c)

	c.ISP = ""
	if err := db.Lookup(ip, c); err != nil {
		log.Errorf("error when looking up ip=%q in isp mmdb: %s", ip, err)
		return ""
	}

	return c.ISP
}

var ispPool = &sync.Pool{New: func() interface{} {
	return new(isp)
}}
