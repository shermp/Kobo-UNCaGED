#!/bin/sh

# This script is used safely start and stop Kobo-UNCaGED.
# It handles entering and exiting USBMS mode, mounting and dismounting the onboard partition, and killing and restoring WiFi

# Some parts of this script, and all scripts in "koreader-scripts" come from Koreader. The source of these scripts is
# https://github.com/koreader/koreader/tree/master/platform/kobo which are licenced under the AGPL 3.0 licence

# Include our USBMS library
KU_DIR="$1"
KU_TMP_DIR="$2"
. ./nickel-usbms.sh

# Abort if the device is currently plugged in, as that's liable to confuse Nickel into actually starting a real USBMS session!
# Which'd probably ultimately cause a crash with our shenanigans...
if [ "$(cat /sys/devices/platform/pmic_battery.1/power_supply/mc13892_bat/status)" = "Charging" ]; then
    # Sleep a bit to lose the race with Nickel's opening of our image
    sleep 2
    logmsg "C" "Device is currently plugged in. Aborting!"
    exit 1
fi

logmsg "N" "Entering USBMS mode..."
logmsg "I" "Inserting USB"
insert_usb

logmsg "I" "Scanning for Button"
BS_TIMEOUT=0
while ! ./button_scan -p; do
    # If the button scan hasn't succeeded in 5 seconds, assume it's not going to...
    if [ $BS_TIMEOUT -ge 20 ]; then
        exit 1
    fi
    # Button scan hasn't succeeded yet. wait a bit (250ms), then try again
    usleep 250000
    BS_TIMEOUT=$(( BS_TIMEOUT + 1 ))
done

logmsg "I" "(Re)mounting onboard"
mount_onboard
ret=$?
if [ ${ret} -ne 0 ]; then
    logmsg "C" "Onboard did not remount (${ret}). Aborting!"
    remove_usb
    exit 1
fi

logmsg "I" "Enabling WiFi"
enable_wifi
ret=$?
if [ ${ret} -ne 0 ]; then
    logmsg "C" "WiFi did not enable (${ret}). Aborting!"
    unmount_onboard
    remove_usb
    exit 1
fi
logmsg "I" "Acquiring IP"
obtain_ip


logmsg "N" "USBMS mode entered . . ."


logmsg "I" "Running Kobo-UNCaGED"
KU_BIN="${MNT_ONBOARD_NEW}/${KU_DIR}/bin/kobo-uncaged"
$KU_BIN "-onboardmount=${MNT_ONBOARD_NEW}"
KU_RES=$?


logmsg "N" "Leaving USBMS . . ."
logmsg "I" "Disabling WiFi"
release_ip
disable_wifi
ret=$?
logmsg "N" "WiFi disabled (${ret}) . . ."

logmsg "I" "Unmounting onboard"
unmount_onboard
ret=$?
logmsg "N" "Onboard unmounted (${ret}) . . ."

logmsg "I" "Waiting for content processing"
./button_scan -w -u -q
BS_RES=$?
if [ $KU_RES -eq 1 ] && [ $BS_RES -eq 0 ]; then
    logmsg "N" "Updating metadata . . ."
    logmsg "I" "Entering USBMS mode . . ."
    insert_usb

    logmsg "I" "Scanning for Button"
    BS_TIMEOUT=0
    while ! ./button_scan -p -q; do
        # If the button scan hasn't succeeded in 5 seconds, assume it's not going to...
        if [ $BS_TIMEOUT -ge 20 ]; then
            exit 1
        fi
        # Button scan hasn't succeeded yet. wait a bit (250ms), then try again
        usleep 250000
        BS_TIMEOUT=$(( BS_TIMEOUT + 1 ))
    done

    logmsg "I" "(Re)mounting onboard"
    mount_onboard
    ret=$?
    if [ ${ret} -ne 0 ]; then
        logmsg "C" "Onboard did not remount (${ret}). Aborting!"
        remove_usb
        exit 1
    fi

    logmsg "I" "Running Kobo-UNCaGED"
    $KU_BIN "-onboardmount=${MNT_ONBOARD_NEW}" "-metadata"

    logmsg "I" "Unmounting onboard"
    unmount_onboard
    ret=$?
    logmsg "N" "Onboard unmounted (${ret}) . . ."

    logmsg "I" "Going back to Nickel"
    remove_usb
else
    # FBInk returns negative error codes, fudge that back to the <errno.h> value...
    logmsg "I" "Nothing more to do? (KU: ${KU_RES}; BS: $(( 256 - BS_RES )))"
fi
