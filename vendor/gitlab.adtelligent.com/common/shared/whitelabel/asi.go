package whitelabel

import "flag"

var (
	defaultASIFlag = flag.String("defaultASI", "adtelligent.com", "default ASI")
)

func GetASI() string {
	return *defaultASIFlag
}
