#!/bin/sh

# A few constants for stuff that should pretty much be set in stone
ONBOARD_BLOCKDEV="/dev/mmcblk0p3"
SDCARD_BLOCKDEV="/dev/mmcblk1p1"

# Logging facilities
logmsg() {
    # Set terminal color escape sequences
    END="\033[0m"
    RED="\033[31;1m"
    YELLOW="\033[33;1m"
    GREEN="\033[32;1m"

    # Set the requested loglevel, default to notice, like logger
    LOG_LEVEL="notice"
    PRINT_COLOR="${GREEN}"
    case "${1}" in
        "C" )
            LOG_LEVEL="crit"
            PRINT_COLOR="${RED}"
        ;;
        "E" )
            LOG_LEVEL="err"
            PRINT_COLOR="${RED}"
        ;;
        "W" )
            LOG_LEVEL="warning"
            PRINT_COLOR="${YELLOW}"
        ;;
        "N" )
            LOG_LEVEL="notice"
            PRINT_COLOR="${YELLOW}"
        ;;
        "I" )
            LOG_LEVEL="info"
            PRINT_COLOR="${GREEN}"
        ;;
        "D" )
            LOG_LEVEL="debug"
            PRINT_COLOR="${YELLOW}"
        ;;
    esac

    # Actual message ;)
    LOG_MSG="${2}"

    # Send to syslog
    logger -t "UNCaGED" -p daemon.${LOG_LEVEL} "${LOG_MSG}"

    # Print to console
    printf "%b%s%b\n" "${PRINT_COLOR}" "${LOG_MSG}" "${END}"

    # Print to screen
    PRINT_ROW=4
    # Keep notices visible by printing them one row higher
    if [ "${LOG_LEVEL}" = "notice" ]; then
        PRINT_ROW=3
    fi

    # Keep verbose debugging off-screen, though...
    if [ "${LOG_LEVEL}" != "debug" ] ; then
        ./fbink -q -y ${PRINT_ROW} -mp "${LOG_MSG}"
    fi
}

# Get the needed environment variables from the running Nickel process.
# Needed by the Wifi enable/disable functions
get_nickel_env() {
    eval "$(xargs -n 1 -0 <"/proc/$(pidof nickel)/environ" | grep -e DBUS_SESSION_BUS_ADDRESS -e NICKEL_HOME -e WIFI_MODULE -e LANG -e WIFI_MODULE_PATH -e INTERFACE 2>/dev/null)"
}

insert_usb() {
    sync
    echo "usb plug add" >> /tmp/nickel-hardware-status
}
remove_usb() {
    sync
    echo "usb plug remove" >> /tmp/nickel-hardware-status
}

# The contents of the following functions were adapted from KOreader
release_ip() {
    get_nickel_env

    # Release IP and shutdown udhcpc.
    pkill -9 -f '/bin/sh /etc/udhcpc.d/default.script'
    ifconfig "${INTERFACE}" 0.0.0.0
}

obtain_ip() {
    get_nickel_env

    # Make sure we start from scratch
    release_ip

    # Use udhcpc to obtain IP.
    env -u LD_LIBRARY_PATH udhcpc -S -i "${INTERFACE}" -s /etc/udhcpc.d/default.script -t15 -T10 -A3 -b -q
}

wifi_is_forced() {
    if grep -qs " /mnt/onboard" "/proc/mounts"; then
        KOBO_CONF_FILE="/mnt/onboard/.kobo/Kobo/Kobo eReader.conf"
    elif grep -qs " /mnt/newonboard" "/proc/mounts"; then
        KOBO_CONF_FILE="/mnt/newonboard/.kobo/Kobo/Kobo eReader.conf"
    else
        return 2
    fi
    grep -qs "ForceWifiOn=true" "$KOBO_CONF_FILE"
    return $?
}

enable_wifi() {
    if wifi_is_forced; then
        return 0
    fi

    get_nickel_env

    WIFI_TIMEOUT=0
    while lsmod | grep -q sdio_wifi_pwr; do
        # If the Wifi hasn't been killed by Nickel within 5 seconds, assume it's not going to...
        if [ ${WIFI_TIMEOUT} -ge 20 ]; then
            return 0
        fi
        # Nickel hasn't killed Wifi yet. We sleep for a bit (250ms), then try again
        usleep 250000
        WIFI_TIMEOUT=$(( WIFI_TIMEOUT + 1 ))
    done

    # Load wifi modules and enable wifi.
    lsmod | grep -q sdio_wifi_pwr || insmod "/drivers/${PLATFORM}/wifi/sdio_wifi_pwr.ko"
    # Moar sleep!
    usleep 250000
    # WIFI_MODULE_PATH = /drivers/$PLATFORM/wifi/$WIFI_MODULE.ko
    lsmod | grep -q "${WIFI_MODULE}" || insmod "${WIFI_MODULE_PATH}"
    # Race-y as hell, don't try to optimize this!
    sleep 1

    ifconfig "${INTERFACE}" up
    [ "$WIFI_MODULE" != "8189fs" ] && [ "${WIFI_MODULE}" != "8192es" ] && wlarm_le -i "${INTERFACE}" up

    pidof wpa_supplicant >/dev/null \
        || env -u LD_LIBRARY_PATH \
            wpa_supplicant -D wext -s -i "${INTERFACE}" -O /var/run/wpa_supplicant -c /etc/wpa_supplicant/wpa_supplicant.conf -B
    
    WIFI_TIMEOUT=0
    while ! wpa_cli status | grep -q "wpa_state=COMPLETED"; do
        # If wpa_supplicant hasn't connected within 5 seconds, we couldn't connect to the Wifi network
        if [ ${WIFI_TIMEOUT} -ge 20 ]; then
            logmsg "E" "wpa_supplicant failed to connect"
            return 1
        fi
        usleep 250000
        WIFI_TIMEOUT=$(( WIFI_TIMEOUT + 1 ))
    done
    
    # Obtain an IP address
    logmsg "I" "Acquiring IP"
    obtain_ip
}

disable_wifi() {
    if wifi_is_forced; then
        return 0
    fi
    # Before we continue, better release our IP
    release_ip

    get_nickel_env

    # Disable wifi, and remove all modules.
    killall udhcpc default.script wpa_supplicant 2>/dev/null

    [ "${WIFI_MODULE}" != "8189fs" ] && [ "${WIFI_MODULE}" != "8192es" ] && wlarm_le -i "${INTERFACE}" down
    ifconfig "${INTERFACE}" down

    # Some sleep in between may avoid system getting hung
    # (we test if a module is actually loaded to avoid unneeded sleeps)
    if lsmod | grep -q "${WIFI_MODULE}"; then
        usleep 250000
        rmmod "${WIFI_MODULE}"
    fi
    if lsmod | grep -q sdio_wifi_pwr; then
        usleep 250000
        rmmod sdio_wifi_pwr
    fi
}

# Call example: mount_fs "$ONBOARD_BLOCKDEV" "/mnt/newonboard"
mount_fs() {
    # Set variables
    BLK_DEV="$1"
    MNT_NEW="$2"
    # Make sure to create the new mountpoint directory, if it doesn't already exist
    mkdir -p "$MNT_NEW"
    # First check to make device isn't already mounted, if so, we keep trying for up to 5 seconds
    # before aborting
    MOUNT_TIMEOUT=0
    while grep -qs "^${BLK_DEV}" "/proc/mounts"; do
        # If the partition is still mounted after 5 seconds, we abort
        if [ ${MOUNT_TIMEOUT} -ge 20 ]; then
            return 254
        fi
        # Nickel hasn't unmounted device yet. We sleep for a bit (250ms), then try again
        usleep 250000
        MOUNT_TIMEOUT=$(( MOUNT_TIMEOUT + 1 ))
    done

    # If we got this far, we are ready to mount
    sleep 1
    msg="$(mount -o noatime,nodiratime,shortname=mixed,utf8 -t vfat "$BLK_DEV" "$MNT_NEW" 2>&1)"
    ret=$?
    if [ ${ret} -ne 0 ]; then
        logmsg "D" "Failed to mount ${BLK_DEV}! (${ret}: ${msg})"
    fi
    return ${ret}
}

# Call exampe: unmount_fs "/mnt/newonboard"
unmount_fs() {
    MNT_NEW="$1"
    # next, make sure we are still mounted where we expect to be
    if ! grep -qs " ${MNT_NEW}" "/proc/mounts"; then
        return 253
    fi

    # If mounted, we now try to unmount
    sync
    sleep 1
    msg="$(umount "$MNT_NEW" 2>&1)"
    ret=$?
    if [ ${ret} -ne 0 ]; then
        logmsg "D" "Failed to unmount ${MNT_NEW}! (${ret}: ${msg})"
    fi
    return ${ret}
}

# Note, no SD card is not considered an error. Always check whether
# MNT_SD_NEW exists before attempting to use the SD card
mount_sd() {
    # Check if SD card is present
    if [ ! -b "$SDCARD_BLOCKDEV" ]; then
        ret=0
    else
        MNT_SD_NEW="/mnt/newsd"
        mount_fs "$SDCARD_BLOCKDEV" "$MNT_SD_NEW"
        ret=$?
        if [ ${ret} -ne 0 ]; then
            unset MNT_SD_NEW
        fi
    fi
    return ${ret}
}

mount_onboard() {
    MNT_ONBOARD_NEW="/mnt/newonboard"
    mount_fs "$ONBOARD_BLOCKDEV" "$MNT_ONBOARD_NEW"
    ret=$?
    if [ ${ret} -ne 0 ]; then
        unset MNT_ONBOARD_NEW
    fi
    return ${ret}
}

unmount_sd() {
    if [ ! -b "$SDCARD_BLOCKDEV" ]; then
        ret=0
    elif [ -z "$MNT_SD_NEW" ]; then
        ret=254
    else
        unmount_fs "$MNT_SD_NEW"
        ret=$?
        unset MNT_SD_NEW
    fi
    return ${ret}
}

unmount_onboard() {
    # First, we check if mount_onboard has previously been invoked
    if [ -z "$MNT_ONBOARD_NEW" ]; then
        ret=254
    else
        unmount_fs "$MNT_ONBOARD_NEW"
        ret=$?
        unset MNT_ONBOARD_NEW
    fi
    return ${ret}
}
