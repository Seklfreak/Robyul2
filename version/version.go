package version

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
func DumpInfo() string {
    return "BOT VERSION: " + BOT_VERSION + "\n" +
        "BUILD TIME: " + BUILD_TIME + "\n" +
        "BUILD USER: " + BUILD_USER + "\n" +
        "BUILD HOST: " + BUILD_HOST + "\n"
}
