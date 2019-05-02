#!/bin/sh

# Set terminal color escape sequences
END="\033[0m"
RED="\033[31;1m"
YELLOW="\033[33;1m"
GREEN="\033[32;1m"

# Check if the user has set their ARM toolchain name
if [ -z "$CROSS_TC" ]; then
    printf "%bCROSS_TC variable is not set! Please set it before running this script! Eg: CROSS_TC=\"arm-kobo-linux-gnueabihf\"%b\n" "${RED}" "${END}"
    exit 1
fi

# Then check if the toolchain is in the $PATH
if ! command -v "${CROSS_TC}-gcc"; then
    printf "%bARM toolchain not found! Please add to PATH\n%b\n" "${RED}" "${END}"
    exit 1
fi

# Set variables required for Go and CGO
# if [ -z "$GOPATH" ]; then
#     printf "GOPATH not found! Please set this before running the script!"
#     exit 1
# fi
export GOOS=linux
export GOARCH=arm
export CGO_ENABLED=1

export CC="${CROSS_TC}-gcc"
export CXX="${CROSS_TC}-g++"

mkdir -p ./Build/onboard/.adds/kobo-uncaged/bin
mkdir -p ./Build/onboard/.adds/kobo-uncaged/fonts
mkdir -p ./Build/onboard/.adds/kobo-uncaged/scripts
mkdir -p ./Build/onboard/.adds/kfmon/config
cd ./Build || exit 1

# Build FBInk
if [ ! -f ./onboard/.adds/kobo-uncaged/bin/fbink ] && [ ! -f ./onboard/.adds/kobo-uncaged/bin/button_scan ]; then
    printf "%bFBInk binaries not found. Building from source%b\n" "${YELLOW}" "${END}"
    if [ ! -d ./FBInk ]; then
        git clone --recursive --branch v1.13.0 https://github.com/NiLuJe/FBInk.git
    fi
    cd ./FBInk || exit 1
    make clean
    # Build the standard build first for button_scan
    if ! make; then
        printf "%bMake failed to build 'button_scan'. Aborting%b\n" "${RED}" "${END}"
        exit 1
    fi
    cp ./Release/button_scan ../onboard/.adds/kobo-uncaged/bin/button_scan
    # Clean for minimal build
    make clean
    if ! make MINIMAL=1; then
        printf "%bMake failed to build 'fbink'. Aborting%b\n" "${RED}" "${END}"
        exit 1
    fi
    cp ./Release/fbink ../onboard/.adds/kobo-uncaged/bin/fbink
    cd -
    printf "%bFBInk binaries built%b\n" "${GREEN}" "${END}"
fi

# Get Go-FBInk-v2
printf "%bGetting Go-FBInk-v2%b\n" "${YELLOW}" "${END}"
if ! go get github.com/shermp/go-fbink-v2/gofbink; then
    printf "%bGo failed to get go-fbink-v2. Aborting%b\n" "${RED}" "${END}"
    exit 1
fi
printf "%bGot Go-FBInk-v2%b\n" "${GREEN}" "${END}"

# Copy the kobo-uncaged scripts to the build directory
cp ../scripts/start-ku.sh ./onboard/.adds/kobo-uncaged/start-ku.sh
cp ../scripts/run-ku.sh ./onboard/.adds/kobo-uncaged/scripts/run-ku.sh
cp ../scripts/nickel-usbms.sh ./onboard/.adds/kobo-uncaged/scripts/nickel-usbms.sh

# And the kfmon files
cp ../kfmon/kobo-uncaged.ini ./onboard/.adds/kfmon/config/kobo-uncaged.ini
cp ../kfmon/Kobo-UNCaGED.png ./onboard/Kobo-UNCaGED.png

# Next, obtain a TTF font. LiberationSans in our case
if [ ! -f ./onboard/.adds/kobo-uncaged/fonts/LiberationSans-Regular.ttf ]; then
    printf "%bFont not found. Downloading LiberationSans%b\n" "${YELLOW}" "${END}"
    wget https://github.com/liberationfonts/liberation-fonts/files/2926169/liberation-fonts-ttf-2.00.5.tar.gz
    tar -zxf ./liberation-fonts-ttf-2.00.5.tar.gz liberation-fonts-ttf-2.00.5/LiberationSans-Regular.ttf
    cp ./liberation-fonts-ttf-2.00.5/LiberationSans-Regular.ttf ./onboard/.adds/kobo-uncaged/fonts/LiberationSans-Regular.ttf
    printf "%bLiberationSans-Regular.ttf downloaded%b\n" "${GREEN}" "${END}"
fi

# Now that we have everything, time to build Kobo-UNCaGED
printf "%bBuilding Kobo-UNCaGED%b\n" "${YELLOW}" "${END}"
cd ./onboard/.adds/kobo-uncaged/bin || exit 1
if ! go build github.com/$(git remote get-url origin | awk '{print $2}' FS='github.com/')/kobo-uncaged; then
    printf "%bGo failed to build kobo-uncaged. Aborting%b\n" "${RED}" "${END}"
    exit 1
fi
"${CROSS_TC}-strip" --strip-unneeded kobo-uncaged
cd -
printf "%bKobo-UNCaGED built%b\n" "${GREEN}" "${END}"

# Finally, zip it all up
printf "%bCreating release archive%b\n" "${YELLOW}" "${END}"
cd ./onboard || exit 1
if ! zip -r ../KoboUncaged.zip .; then
    printf "%bFailed to create zip archive. Aborting%b\n" "${RED}" "${END}"
    exit 1
fi
cd -
printf "%b./Build/KoboUncaged.zip built%b\n" "${GREEN}" "${END}"
