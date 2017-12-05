#!/usr/bin/env bash

set -e

BOT_VERSION=$(git describe --tags)
BUILD_TIME=$(date +%T-%D)
BUILD_USER=$(whoami)
BUILD_HOST=$(hostname)

if [[ "$CI" == "true" ]]; then
    GOTARGET="${GOTARGET?:'A target is mandatory'}"
else
    GOTARGET="Robyul2"
fi

set -x

patch vendor/github.com/bwmarrin/discordgo/restapi.go patches/adds_robyul_discord_rest_api_logging.patch -R

go-bindata -nomemcopy -nocompress -pkg helpers -o helpers/assets.go _assets/

go build ${@} \
    -o "${GOTARGET}" \
    --ldflags="
-X github.com/Seklfreak/Robyul2/version.BOT_VERSION=${BOT_VERSION}
-X github.com/Seklfreak/Robyul2/version.BUILD_TIME=${BUILD_TIME}
-X github.com/Seklfreak/Robyul2/version.BUILD_USER=${BUILD_USER}
-X github.com/Seklfreak/Robyul2/version.BUILD_HOST=${BUILD_HOST}" .

