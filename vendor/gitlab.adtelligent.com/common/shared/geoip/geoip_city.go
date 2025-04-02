package geoip

import (
	"sync"
)

type countryAndCity struct {
	Country struct {
		ISOCode   string `maxminddb:"iso_code"`
		GeonameID uint32 `maxminddb:"geoname_id"`
	} `maxminddb:"country"`
	Regions []struct {
		ISOCode   string `maxminddb:"iso_code"`
		GeonameID uint32 `maxminddb:"geoname_id"`
	} `maxminddb:"subdivisions"`
	City struct {
		GeonameID uint32            `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
		Metro     uint32  `maxminddb:"metro_code"`
		Accuracy  uint32  `maxminddb:"accuracy_radius"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
}

var countryAndCityPool = &sync.Pool{
	New: func() interface{} {
		return &countryAndCity{}
	},
}

func (c *countryAndCity) reset() {
	// reset country code to ZZ, since db.Lookup doesn't set it
	// if the country code cannot be found.
	c.Country.ISOCode = "ZZ"
	c.Country.GeonameID = 0
	c.Regions = c.Regions[:0]
	c.City.GeonameID = 0
	if c.City.Names != nil {
		c.City.Names["en"] = ""
	}
	c.Location.Latitude = 0
	c.Location.Longitude = 0
	c.Location.TimeZone = ""
	c.Postal.Code = ""
	c.Location.Accuracy = 0
	c.Location.Metro = 0
}
