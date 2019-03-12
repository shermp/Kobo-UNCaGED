#!/bin/sh

# This script is used safely start and stop Kobo-UNCaGED.
# It handles entering and exiting USBMS mode, mounting and dismounting the onboard partition, and killing and restoring WiFi

# Some parts of this script, and all scripts in "koreader-scripts" come from Koreader. The source of these scripts is
# https://github.com/koreader/koreader/tree/master/platform/kobo which are licenced under the AGPL 3.0 licence

# Include our USBMS library
KU_DIR="$1"
KU_TMP_DIR="$2"
. ./nickel-usbms.sh

./fbink -y 0 -Y 100 -m -p -r "Entering USBMS mode..."
insert_usb

BS_TIMEOUT=0
while ! ./button_scan -p
do
    # If the button scan hasn't succeeded in 5 seconds, assume it's not going to...
    if [ $BS_TIMEOUT -ge 20 ]; then
        return 0
    fi
    # Button scan hasn't succeeded yet. wait a bit (250ms), then try again
    usleep 250000
    BS_TIMEOUT=$(( BS_TIMEOUT + 1 ))
done

if ! mount_onboard; then
    remove_usb
fi
if ! enable_wifi; then
    unmount_onboard
    remove_usb
fi

./fbink -y 0 -Y 100 -m -p -r "USBMS mode entered..."

sleep 10

./fbink -y 0 -Y 100 -m -p -r "Leaving USBMS..."
disable_wifi
./fbink -y 0 -Y 100 -m -p -r "Wifi Disabled..."
unmount_onboard
./fbink -y 0 -Y 100 -m -p -r "Onboard Unmounted..."
remove_usb
    
