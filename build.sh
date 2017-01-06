#!/usr/bin/env bash

BOT_VERSION=$(git describe --tags)
BUILD_TIME=$(date +%T-%D)
BUILD_USER="$USER"
BUILD_HOST=$(hostname)

go build --ldflags="
-X github.com/sn0w/Karen/version.BOT_VERSION=${BOT_VERSION}
-X github.com/sn0w/Karen/version.BUILD_TIME=${BUILD_TIME}
-X github.com/sn0w/Karen/version.BUILD_USER=${BUILD_USER}
-X github.com/sn0w/Karen/version.BUILD_HOST=${BUILD_HOST}" -o ${GOTARGET?:"Set a target"} .
