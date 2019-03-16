#!/bin/sh

# Set terminal color escape sequences
END="\033[0m"
RED="\033[31;1m"
YELLOW="\033[33;1m"
GREEN="\033[32;1m"

# Check if the user has set their ARM toolchain name
if [ -z "$CROSS_TC" ]; then
    printf "${RED}CROSS_TC variable is not set! Please set it before running this script! Eg: CROSS_TC=\"arm-kobo-linux-gnueabihf\"${END}\n"
    exit 1
fi

# Then check if the toolchain is in the $PATH
if ! which "${CROSS_TC}-gcc"; then
    printf "${RED}ARM toolchain not found! Please add to PATH\n${END}\n"
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
cd ./Build

# Build FBInk
if [ ! -f ./onboard/.adds/kobo-uncaged/bin/fbink ] && [ ! -f ./onboard/.adds/kobo-uncaged/bin/button_scan ]; then
    printf "${YELLOW}FBInk binaries not found. Building from source${END}\n"
    if [ ! -d ./FBInk ]; then
        git clone --recursive --branch v1.12.1 https://github.com/NiLuJe/FBInk.git
    fi
    cd ./FBInk
    make clean
    # Build the standard build first for button_scan
    if ! make; then
        printf "${RED}Make failed to build 'button_scan'. Aborting${END}\n"
        exit 1
    fi
    cp ./Release/button_scan ../onboard/.adds/kobo-uncaged/bin/button_scan
    # Clean for minimal build
    make clean
    if ! MINIMAL=1 make; then
        printf "${RED}Make failed to build 'fbink'. Aborting${END}\n"
        exit 1
    fi
    cp ./Release/fbink ../onboard/.adds/kobo-uncaged/bin/fbink
    cd ..
    printf "${GREEN}FBInk binaries built${END}\n"
fi
# Copy the kobo-uncaged scripts to the build directory
cp ../scripts/start-ku.sh ./onboard/.adds/kobo-uncaged/start-ku.sh
cp ../scripts/run-ku.sh ./onboard/.adds/kobo-uncaged/scripts/run-ku.sh
cp ../scripts/nickel-usbms.sh ./onboard/.adds/kobo-uncaged/scripts/nickel-usbms.sh

# Next, obtain a TTF font. LiberationSans in our case
if [ ! -f ./onboard/.adds/kobo-uncaged/fonts/LiberationSans-Regular.ttf ]; then
    printf "${YELLOW}Font not found. Downloading LiberationSans${END}\n"
    wget https://github.com/liberationfonts/liberation-fonts/files/2926169/liberation-fonts-ttf-2.00.5.tar.gz
    tar -zxf ./liberation-fonts-ttf-2.00.5.tar.gz liberation-fonts-ttf-2.00.5/LiberationSans-Regular.ttf
    cp ./liberation-fonts-ttf-2.00.5/LiberationSans-Regular.ttf ./onboard/.adds/kobo-uncaged/fonts/LiberationSans-Regular.ttf
    printf "${GREEN}LiberationSans-Regular.ttf downloaded${END}\n"
fi

# Now that we have everything, time to build Kobo-UNCaGED
printf "${YELLOW}Building Kobo-UNCaGED${END}\n"
cd ./onboard/.adds/kobo-uncaged/bin
if ! go build github.com/shermp/Kobo-UNCaGED/kobo-uncaged; then
    printf "${RED}Go failed to build kobo-uncaged. Aborting${END}\n"
    exit 1
fi
$CROSS_TC-strip kobo-uncaged
cd ../../../../
printf "${GREEN}Kobo-UNCaGED built${END}\n"

# Finally, zip it all up
printf "${YELLOW}Creating release archive${END}\n"
cd ./onboard
if ! zip -r ../KoboUncaged.zip .; then
    printf "${RED}Failed to create zip archive. Aborting${END}\n"
    exit 1
fi
cd ..
printf "${GREEN}./Build/KoboUncaged.zip built${END}\n"