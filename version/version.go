package version

import "github.com/Seklfreak/Robyul2/logger"

// Version related vars
// Set by compiler
var (
    // BOT_VERSION example: 0.5.2-4-g205bbb8
    BOT_VERSION string = "DEV_SNAPSHOT"

    // BUILD_TIME example: Fri Jan  6 00:45:46 CET 2017
    BUILD_TIME string = "UNSET"

    // BUILD_USER example: sn0w
    BUILD_USER string = "UNSET"

    // BUILD_HOST example: nepgear
    BUILD_HOST string = "UNSET"
)

// DumpInfo dumps all above vars
func DumpInfo() {
    logger.VERBOSE.L("version", "BOT VERSION: "+BOT_VERSION)
    logger.VERBOSE.L("version", "BUILD TIME: "+BUILD_TIME)
    logger.VERBOSE.L("version", "BUILD USER: "+BUILD_USER)
    logger.VERBOSE.L("version", "BUILD HOST: "+BUILD_HOST)
}
