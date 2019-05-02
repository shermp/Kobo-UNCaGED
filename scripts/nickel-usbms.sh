#!/bin/sh

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

    ./fbink -q -y ${PRINT_ROW} -mpr "${LOG_MSG}"
}

# Get the needed environment variables from the running Nickel process.
# Needed by the Wifi enable/disable functions
get_nickel_env() {
    eval "$(xargs -n 1 -0 <"/proc/$(pidof nickel)/environ" | grep -e DBUS_SESSION_BUS_ADDRESS -e NICKEL_HOME -e WIFI_MODULE -e LANG -e WIFI_MODULE_PATH -e INTERFACE 2>/dev/null)"
}

insert_usb() {
    echo "usb plug add" >> /tmp/nickel-hardware-status
}
remove_usb() {
    echo "usb plug remove" >> /tmp/nickel-hardware-status
}

# The contents of the following function were adapted from KOreader
release_ip() {
    get_nickel_env
    # Release IP and shutdown udhcpc.
    pkill -9 -f '/bin/sh /etc/udhcpc.d/default.script'
    ifconfig "${INTERFACE}" 0.0.0.0
}

obtain_ip() {
    get_nickel_env
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
    while lsmod | grep -q sdio_wifi_pwr
    do
        # If the Wifi hasn't been killed by Nickel within 5 seconds, assume it's not going to...
        if [ $WIFI_TIMEOUT -ge 20 ]; then
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
}

disable_wifi() {
    if wifi_is_forced; then
        return 0
    fi
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

mount_onboard() {
    # Set mountpoint variables
    MNT_ONBOARD_NEW="/mnt/newonboard"
    # Make sure to create the new mountpoint directory, if it doesn't already exist
    mkdir -p "$MNT_ONBOARD_NEW"
    # First check to make sure onboard isn't already mounted, if so, we keep trying for up to 5 seconds
    # before aborting
    MOUNT_TIMEOUT=0
    while grep -qs "^/dev/mmcblk0p3" "/proc/mounts"; do
        # If the partition is still mounted after 5 seconds, we abort
        if [ $MOUNT_TIMEOUT -ge 20 ]; then
            return 255
        fi
        # Nickel hasn't unmounted /dev/mmcblk0p3 yet. We sleep for a bit (250ms), then try again
        usleep 250000
        MOUNT_TIMEOUT=$(( MOUNT_TIMEOUT + 1 ))
    done
    # If we got this far, we are ready to mount
    mount -t vfat -o rw,noatime,nodiratime,fmask=0022,dmask=0022,codepage=cp437,iocharset=iso8859-1,shortname=mixed,utf8 /dev/mmcblk0p3 "$MNT_ONBOARD_NEW"
    return $?
}

unmount_onboard() {
    # First, we check if mount_onboard has previously been invoked
    if [ -z "$MNT_ONBOARD_NEW" ]; then
        return 255
    fi
    # next, make sure we are still mounted where we expect to be
    if ! grep -qs " $MNT_ONBOARD_NEW" "/proc/mounts"; then
        return 254
    fi
    # If mounted, we now try to unmount
    umount "$MNT_ONBOARD_NEW"
    return $?
}
