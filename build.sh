#!/usr/bin/env bash

set -e

BOT_VERSION=$(git describe --tags)
BUILD_TIME=$(date +%T-%D)
BUILD_USER="$USER"
BUILD_HOST=$(hostname)
XFLAGS="-v"

if [[ "$CI" == "true" ]]; then
    GOTARGET="${GOTARGET?:'Set a target'}"
    XFLAGS="-x $XFLAGS"
else
    GOTARGET="Karen"
fi

echo $GOTARGET

go build ${XFLAGS} --ldflags="
-X git.lukas.moe/sn0w/Karen/version.BOT_VERSION=${BOT_VERSION}
-X git.lukas.moe/sn0w/Karen/version.BUILD_TIME=${BUILD_TIME}
-X git.lukas.moe/sn0w/Karen/version.BUILD_USER=${BUILD_USER}
-X git.lukas.moe/sn0w/Karen/version.BUILD_HOST=${BUILD_HOST}" -o ${GOTARGET} .
