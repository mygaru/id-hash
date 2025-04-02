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

const (
	ConnTypeDialup = "Dialup"
	ConnTypeCable  = "Cable/DSL"
	ConnTypeCorp   = "Corporate"
	ConnTypeCell   = "Cellular"
)

var (
	connTypeFilePath = flag.String("connTypeFilePath", "", "Path to maxmind connection type database")
)

type connectionType struct {
	ConnectionType string `maxminddb:"connection_type"`
}

func connTypeOpen() {
	if *connTypeFilePath == "" {
		return
	}

	log.Infof("opening mmdb file at %q", *connTypeFilePath)

	f, err := os.Open(*connTypeFilePath)
	if err != nil {
		log.Errorf("cannot open mmdb file at %q: %s", *connTypeFilePath, err)
		return
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Errorf("error when reading mmdb file at %q: %s", *connTypeFilePath, err)
		return
	}

	db, err := maxminddb.FromBytes(data)
	if err != nil {
		log.Errorf("error when opening mmdb at %q: %s", *connTypeFilePath, err)
		return
	}
	log.Infof("verifying opened mmdb file...")
	if err = db.Verify(); err != nil {
		log.Errorf("incorrect mmdb file at %q: %s", *connTypeFilePath, err)
		return
	}
	log.Infof("successfully verified mmdb file")

	connTypeDb.Store(db)
}

var connTypeDb atomic.Value

func ConnectionTypeByIP(ip net.IP) string {
	db, ok := connTypeDb.Load().(*maxminddb.Reader)
	if !ok || db == nil {
		return ""
	}
	c := connTypePool.Get().(*connectionType)
	defer connTypePool.Put(c)

	c.ConnectionType = ""
	if err := db.Lookup(ip, c); err != nil {
		log.Errorf("error when looking up ip=%q in connType mmdb: %s", ip, err)
		return ""
	}

	return c.ConnectionType
}

var connTypePool = &sync.Pool{New: func() interface{} {
	return new(connectionType)
}}
