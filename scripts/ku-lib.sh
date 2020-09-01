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

    # # Print to screen
    # PRINT_ROW=4

    # Print warnings and errors to screen
    # if [ "${LOG_LEVEL}" != "debug" ] && [ "${LOG_LEVEL}" != "info" ] ; then
    #     $FBINK_BIN -q -y ${PRINT_ROW} -mp "${LOG_MSG}"
    # fi
}

# Check that the current firmware satisfies the argument passed into
# this function. There is no error checking at the moment.
fw_satisfies() {
    min_fw="$1"

    oldifs="$IFS"
    IFS=','
    read -r serial v2 fw v4 v5 model < /mnt/onboard/.kobo/version
    IFS='.'
    read -r cur_maj cur_min cur_build <<EOF
$fw
EOF
    read -r min_maj min_min min_build <<EOF
$min_fw
EOF

    IFS="oldifs"
    # Make sure that build only contains digits
    cur_build=$(expr "${cur_build}" : '\([0-9][0-9]*\)')
    min_build=$(expr "${min_build}" : '\([0-9][0-9]*\)')

    if [ "$cur_maj" -gt "$min_maj" ] ; then
        return 0
    elif [ "$cur_maj" -eq "$min_maj" ] ; then
        if [ "$cur_min" -gt "$min_min" ] ; then
            return 0
        elif [ "$cur_min" -eq "$min_min" ] ; then
            if [ "$cur_build" -ge "$min_build" ] ; then
                return 0
            fi
        fi
    fi
    return 1
}

# Checks to see if NickelDBus and qndb is installed, and that the
# NickelDBus version can be obtailed via qndb
ndb_installed() {
    if (which qndb > /dev/null 2>&1) && (qndb -m ndbVersion > /dev/null 2>&1) ; then
        return 0
    else
        logmsg "E" "NickelDBus not found or unable to get version"
        return 1
    fi
}
