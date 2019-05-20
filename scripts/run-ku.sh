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

logmsg "I" "Mounting onboard"
mount_onboard
ret=$?
if [ ${ret} -ne 0 ]; then
    logmsg "C" "Onboard did not mount (${ret}). Aborting!"
    remove_usb
    exit 1
fi
logmsg "I" "Mounting SD card"
mount_sd
ret=$?
if [ ${ret} -ne 0 ]; then
    logmsg "C" "SD card did not mount (${ret}). Aborting!"
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

logmsg "N" "USBMS mode entered . . ."


logmsg "I" "Running Kobo-UNCaGED"
KU_BIN="${MNT_ONBOARD_NEW}/${KU_DIR}/bin/kobo-uncaged"
if [ -z "$MNT_SD_NEW" ]; then
    $KU_BIN -onboardmount="${MNT_ONBOARD_NEW}"
    KU_RES=$?
else
    $KU_BIN -onboardmount="${MNT_ONBOARD_NEW}" -sdmount="${MNT_SD_NEW}"
    KU_RES=$?
fi

logmsg "N" "Leaving USBMS . . ."
logmsg "I" "Disabling WiFi"
disable_wifi
ret=$?
logmsg "N" "WiFi disabled (${ret}) . . ."

logmsg "I" "Unmounting onboard"
unmount_onboard
ret=$?
logmsg "N" "Onboard unmounted (${ret}) . . ."

logmsg "I" "Unmounting SD card"
unmount_sd
ret=$?
logmsg "N" "SD card unmounted (${ret}) . . ."

logmsg "I" "Waiting for content processing"
./button_scan -w -u -q
BS_RES=$?
# Note, KU may have updated metadata, even if no new books are added
if [ $KU_RES -eq 1 ] || [ $BS_RES -eq 0 ]; then
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
    logmsg "I" "Remounting SD card"
    mount_sd
    ret=$?
    if [ ${ret} -ne 0 ]; then
        logmsg "C" "SD card did not Remount (${ret}). Aborting!"
        remove_usb
        exit 1
    fi

    logmsg "I" "Running Kobo-UNCaGED"
    if [ -z "$MNT_SD_NEW" ]; then
        $KU_BIN -onboardmount="${MNT_ONBOARD_NEW}" -metadata
    else
        $KU_BIN -onboardmount="${MNT_ONBOARD_NEW}" -sdmount="${MNT_SD_NEW}" -metadata
    fi

    logmsg "I" "Unmounting onboard"
    unmount_onboard
    ret=$?
    logmsg "N" "Onboard unmounted (${ret}) . . ."

    logmsg "I" "Unmounting SD card"
    unmount_sd
    ret=$?
    logmsg "N" "SD card unmounted (${ret}) . . ."

    logmsg "I" "Going back to Nickel"
    remove_usb
elif [ $KU_RES -eq 100 ]; then
    logmsg "I" "Password issue. Check your ku.toml config file"
elif [ $KU_RES -ne 0 ] && [ $BS_RES -ne 0 ]; then
    # FBInk returns negative error codes, fudge that back to the <errno.h> value...
    logmsg "I" "Something strange happened... (KU: ${KU_RES}; BS: -$(( 256 - BS_RES )))"
fi
