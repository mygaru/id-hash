package instance

import (
	"flag"
	"fmt"
	"gitlab.adtelligent.com/common/shared/geoip"
	"gitlab.adtelligent.com/common/shared/log"
	"gitlab.adtelligent.com/common/shared/metric"
	"gitlab.adtelligent.com/common/shared/util"
	"os"
	"runtime"
	"time"
)

var (
	Group         = flag.String("serverGroup", "Unknown", "Server group")
	ServerZone    = flag.String("serverZone", "ZZ", "us-east,us-west,eu,asia")
	NumberNumeric = flag.Int("serverNumber", 0, "Server ID")
	Prefix        = flag.String("serverPrefix", "ads", "Server prefix")
	NumberString  string
)

func Init() {

	NumberString = fmt.Sprintf(*Prefix+`%d`, *NumberNumeric)

	geoip.OnReady(func() {
		instance := metric.NewCounterVec("instance")

		city := geoip.City{}
		ip := util.ExternalIP()
		geoip.CityByIp(&city, ip)

		zone := *ServerZone
		if zone == "ZZ" {
			zone = city.Zone()
		}

		hostname, _ := os.Hostname()

		instance.With(fmt.Sprintf(`rev="%s",ver="%s",group="%s",id="%s",ip="%s",zone="%s",cores="%d",hostname="%s"`,
			log.GetBuildRevision(),
			log.GetBuildVersion(),
			*Group,
			NumberString,
			ip,
			zone,
			runtime.NumCPU(),
			hostname,
		)).Add(int(time.Now().Unix()))
	})
}
