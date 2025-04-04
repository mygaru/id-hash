package geoip

import (
	"github.com/oschwald/maxminddb-golang"
	"gitlab.adtelligent.com/common/shared/log"
	"net"
	"sync"
)

type City struct {
	Country   *Country
	Name      string
	GeonameID uint32
	Region    struct {
		GeonameID uint32
		ISOCode   string
	}
	Latitude  float64
	Longitude float64
	ZipCode   string
	TimeZone  string
	Metro     uint32
	Accuracy  uint32
}

var cityPool = sync.Pool{New: func() interface{} {
	return new(City)
}}

func AcquireCity() *City {
	return cityPool.Get().(*City)
}

func ReleaseCity(c *City) {
	ResetCity(c)
	cityPool.Put(c)
}

func ResetCity(c *City) {
	c.Country = nil
	c.Name = ""
	c.GeonameID = 0
	c.Region.GeonameID = 0
	c.Region.ISOCode = ""
	c.Latitude = 0
	c.Longitude = 0
	c.ZipCode = ""
	c.TimeZone = ""
	c.Metro = 0
	c.Accuracy = 0
}

func CityByIp(dest *City, ip net.IP) {
	c := countryAndCityPool.Get().(*countryAndCity)
	c.reset()

	db := geoipdb.Load().(*maxminddb.Reader)
	if err := db.Lookup(ip, c); err != nil {
		log.Fatalf("error when looking up ip=%q in geoipdb: %s", ip, err)
	}

	dest.Country = CountryByCode(c.Country.ISOCode)
	dest.GeonameID = c.City.GeonameID
	if len(c.Regions) > 0 {
		dest.Region.ISOCode = c.Regions[0].ISOCode
		dest.Region.GeonameID = c.Regions[0].GeonameID
	}

	dest.Name = c.City.Names["en"]
	dest.Latitude = c.Location.Latitude
	dest.Longitude = c.Location.Longitude
	dest.ZipCode = c.Postal.Code
	dest.TimeZone = c.Location.TimeZone
	dest.Metro = c.Location.Metro
	dest.Accuracy = c.Location.Accuracy

	countryAndCityPool.Put(c)
}
