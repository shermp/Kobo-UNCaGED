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
printf "%bInserting USB%b\n" "${GREEN}" "${END}"
insert_usb

printf "%bScanning for Button%b\n" "${GREEN}" "${END}"
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
printf "%b(Re)mounting onboard%b\n" "${GREEN}" "${END}"
if ! mount_onboard; then
    printf "%bOnboard did not remount%b\n" "${RED}" "${END}"
    remove_usb
    exit 1

fi
printf "%bEnabling WiFi%b\n" "${GREEN}" "${END}"
if ! enable_wifi; then
    printf "%bWiFi did not enable. Aborting!%b\n" "${RED}" "${END}"
    unmount_onboard
    remove_usb
    exit 1
fi

./fbink -y 0 -Y 100 -m -p -r -q "USBMS mode entered..."

printf "%bRunning Kobo-UNCaGED%b\n" "${GREEN}" "${END}"
KU_BIN="${MNT_ONBOARD_NEW}/${KU_DIR}/bin/kobo-uncaged"
$KU_BIN "-onboardmount=${MNT_ONBOARD_NEW}"
KU_RES=$?
printf "%bLeaving USBMS%b\n" "${GREEN}" "${END}"
./fbink -y 0 -Y 100 -m -p -r "Leaving USBMS..."
printf "%bDisabling WiFi%b\n" "${GREEN}" "${END}"
disable_wifi
./fbink -y 0 -Y 100 -m -p -r "Wifi Disabled..."
printf "%bUnmounting onboard%b\n" "${GREEN}" "${END}"
unmount_onboard
./fbink -y 0 -Y 100 -m -p -r "Onboard Unmounted..."

./button_scan -w -u -q
BS_RES=$?
if [ $KU_RES -eq 1 ] && $BS_RES; then
    printf "%bUpdating metadata%b\n" "${GREEN}" "${END}"
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

