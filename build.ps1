$BOT_VERSION=$(git describe --tags)
$BUILD_TIME=$(Get-Date -UFormat "%T-%D")
$BUILD_USER=$env:UserName
$BUILD_HOST=$env:ComputerName
$TARGET=""

if (Get-Variable GOTARGET -Scope Global -ErrorAction SilentlyContinue) {
    $TARGET = $env:GOTARGET
} else {
    $TARGET = "karen.exe"
}

go-bindata -nomemcopy -nocompress -pkg helpers -o helpers/assets.go _assets/

go build $args `
    -o "${TARGET}" `
    --ldflags="
-X git.lukas.moe/sn0w/Karen/version.BOT_VERSION=${BOT_VERSION}
-X git.lukas.moe/sn0w/Karen/version.BUILD_TIME=${BUILD_TIME}
-X git.lukas.moe/sn0w/Karen/version.BUILD_USER=${BUILD_USER}
-X git.lukas.moe/sn0w/Karen/version.BUILD_HOST=${BUILD_HOST}" .
