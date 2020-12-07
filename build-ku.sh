#!/bin/sh

# Allow script to be run from another dir
cd "$(dirname "$0")"

# Logging facilities
logmsg() {
    # Set terminal color escape sequences
    END="\033[0m"
    RED="\033[31;1m"
    YELLOW="\033[33;1m"
    GREEN="\033[32;1m"

    # Set the requested loglevel, default to notice, like logger
    PRINT_COLOR="${GREEN}"
    case "${1}" in
        "E" )
            PRINT_COLOR="${RED}"
        ;;
        "N" )
            PRINT_COLOR="${YELLOW}"
        ;;
        "I" )
            PRINT_COLOR="${GREEN}"
        ;;
    esac

    # Actual message ;)
    LOG_MSG="${2}"

    # Print to console
    printf "%b%s%b\n" "${PRINT_COLOR}" "${LOG_MSG}" "${END}"
}

# Check if the user has set their ARM toolchain name
if [ -z "$CROSS_TC" ] && [ -z "$CROSS_COMPILE" ]; then
    logmsg "E" "CROSS_TC or CROSS_COMPILE variable is not set! Please set it before running this script!\nEg:\nCROSS_TC=\"arm-kobo-linux-gnueabihf\" or\nCROSS_COMPILE=\"arm-kobo-linux-gnueabihf-\""
    exit 1
fi

# Then check if the toolchain is in the $PATH
if ! command -v "${CROSS_TC}-gcc" && ! command -v "${CROSS_COMPILE}gcc"; then
    logmsg "E" "ARM toolchain not found! Please add to PATH\n"
    exit 1
fi

export GOOS=linux
export GOARCH=arm
export CGO_ENABLED=1

if [ -z "$CROSS_COMPILE" ]; then
    export CC="${CROSS_TC}-gcc"
    export CXX="${CROSS_TC}-g++"
else
    export CC="${CROSS_COMPILE}gcc"
    export CXX="${CROSS_COMPILE}g++"
fi

# Setup out directory structure
mkdir -p ./Build/prerequisites/output
rm -rf ./Build/onboard

mkdir -p ./Build/onboard/.adds/kobo-uncaged/bin
mkdir -p ./Build/onboard/.adds/kobo-uncaged/scripts
mkdir -p ./Build/onboard/.adds/kobo-uncaged/config
mkdir -p ./Build/onboard/.adds/kobo-uncaged/templates
mkdir -p ./Build/onboard/.adds/kobo-uncaged/NickelDBus

mkdir -p ./Build/onboard/.adds/nm

cd ./Build/prerequisites || exit 1

# Retrieve NickelDBus, if required
if [ ! -f ./output/ndb-kr.tgz ] ; then
    wget -O ./output/ndb-kr.tgz https://github.com/shermp/NickelDBus/releases/download/0.1.0/KoboRoot.tgz
fi

# Retrieve and build SQLite, if required
SQLITE_VER=sqlite-amalgamation-3330000
if [ ! -f ./$SQLITE_VER/sqlite3 ] ; then
    logmsg "N" "SQLite binary not found. Building from source"
    [ -f ./$SQLITE_VER.zip ] || wget https://www.sqlite.org/2020/$SQLITE_VER.zip
    unzip ./$SQLITE_VER.zip
    $CC -DSQLITE_THREADSAFE=0 \
        -DSQLITE_OMIT_LOAD_EXTENSION \
        -O2 \
        $SQLITE_VER/shell.c $SQLITE_VER/sqlite3.c -o $SQLITE_VER/sqlite3
fi

# Back to the top level Build directory
cd ..
# Now that we have everything, time to build Kobo-UNCaGED
logmsg "N" "Building Kobo-UNCaGED"
cd ./onboard/.adds/kobo-uncaged/bin || exit 1
ku_vers="$(git describe --tags)"
go_ldflags="-s -w -X main.kuVersion=${ku_vers}"
if ! go build -ldflags "$go_ldflags" ../../../../../kobo-uncaged; then
    logmsg "E" "Go failed to build kobo-uncaged. Aborting"
    exit 1
fi
cd -
logmsg "I" "Kobo-UNCaGED built"

# Copy the kobo-uncaged scripts to the build directory
cp ../scripts/nm-start-ku.sh ./onboard/.adds/kobo-uncaged/nm-start-ku.sh
cp ../scripts/ku-prereq-check.sh ./onboard/.adds/kobo-uncaged/scripts/ku-prereq-check.sh
cp ../scripts/ku-lib.sh ./onboard/.adds/kobo-uncaged/scripts/ku-lib.sh

# NickelMenu config file
cp ../config/nm-ku ./onboard/.adds/nm/kobo_uncaged

# SQLite binary
cp ./prerequisites/$SQLITE_VER/sqlite3 ./onboard/.adds/kobo-uncaged/bin/sqlite3

# NickelDBus KoboRoot
cp ./prerequisites/output/ndb-kr.tgz ./onboard/.adds/kobo-uncaged/NickelDBus/ndb-kr.tgz

# HTML templates
cp -r ../kobo-uncaged/templates/. ./onboard/.adds/kobo-uncaged/templates/

# Web UI static files (CSS, Javascript etc)
cp -r ../kobo-uncaged/static/. ./onboard/.adds/kobo-uncaged/static/

# Finally, zip it all up
logmsg "N" "Creating release archive"
cd ./onboard || exit 1
#if ! zip -r "../KoboUncaged-${ku_vers}-${BUILD_TYPE}.zip" .; then
if ! zip -r "../KoboUncaged-${ku_vers}.zip" .; then
    logmsg "E" "Failed to create zip archive. Aborting"
    exit 1
fi
cd -

logmsg "I" "./Build/KoboUncaged-${ku_vers}.zip built"
