#!/bin/sh

# This script is used safely start and stop Kobo-UNCaGED.
# It handles entering and exiting USBMS mode, mounting and dismounting the onboard partition, and killing and restoring WiFi

# Some parts of this script, and all scripts in "koreader-scripts" come from Koreader. The source of these scripts is
# https://github.com/koreader/koreader/tree/master/platform/kobo which are licenced under the AGPL 3.0 licence

# Include our USBMS library
KU_DIR="$1"
KU_TMP_DIR="$2"
. ./nickel-usbms.sh
# Set terminal color escape sequences
END="\033[0m"
RED="\033[31;1m"
YELLOW="\033[33;1m"
GREEN="\033[32;1m"

./fbink -y 0 -Y 100 -m -p -r -q "Entering USBMS mode..."
printf "${GREEN}Inserting USB${END}\n"
insert_usb

printf "${GREEN}Scanning for Button${END}\n"
BS_TIMEOUT=0
while ! ./button_scan -p
do
    # If the button scan hasn't succeeded in 5 seconds, assume it's not going to...
    if [ $BS_TIMEOUT -ge 20 ]; then
        exit 1
    fi
    # Button scan hasn't succeeded yet. wait a bit (250ms), then try again
    usleep 250000
    BS_TIMEOUT=$(( BS_TIMEOUT + 1 ))
done
printf "${GREEN}(Re)mounting onboard${END}\n"
if ! mount_onboard; then
    printf "${RED}Onboard did not remount${END}\n"
    remove_usb
    exit 1

fi
printf "${GREEN}Enabling WiFi${END}\n"
if ! enable_wifi; then
    printf "${RED}WiFi did not enable. Aborting!${END}\n"
    unmount_onboard
    remove_usb
    exit 1
fi

./fbink -y 0 -Y 100 -m -p -r -q "USBMS mode entered..."

printf "${GREEN}Running Kobo-UNCaGED${END}\n"
KU_BIN="${MNT_ONBOARD_NEW}/${KU_DIR}/bin/kobo-uncaged"
$KU_BIN "-onboardmount=${MNT_ONBOARD_NEW}"
KU_RES=$?
printf "${GREEN}Leaving USBMS${END}\n"
./fbink -y 0 -Y 100 -m -p -r "Leaving USBMS..."
printf "${GREEN}Disabling WiFi${END}\n"
disable_wifi
./fbink -y 0 -Y 100 -m -p -r "Wifi Disabled..."
printf "${GREEN}Unmounting onboard${END}\n"
unmount_onboard
./fbink -y 0 -Y 100 -m -p -r "Onboard Unmounted..."

./button_scan -w -u -q
BS_RES=$?
if [ $KU_RES -eq 1 ] && $BS_RES; then
    printf "${GREEN}Updating metadata${END}\n"
    ./fbink -y 0 -Y 100 -m -p -r -q "Entering USBMS mode..."
    insert_usb

    BS_TIMEOUT=0
    while ! ./button_scan -p -q
    do
        # If the button scan hasn't succeeded in 5 seconds, assume it's not going to...
        if [ $BS_TIMEOUT -ge 20 ]; then
            exit 1
        fi
        # Button scan hasn't succeeded yet. wait a bit (250ms), then try again
        usleep 250000
        BS_TIMEOUT=$(( BS_TIMEOUT + 1 ))
    done
    if ! mount_onboard; then
        remove_usb
        exit 1
    fi
    $KU_BIN "-onboardmount=${MNT_ONBOARD_NEW} -metadata"
    unmount_onboard
    ./fbink -y 0 -Y 100 -m -p -r -q "Onboard Unmounted..."
    remove_usb
fi
    
