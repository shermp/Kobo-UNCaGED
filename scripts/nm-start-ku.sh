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

    # Keep verbose debugging off-screen, though...
    if [ "${LOG_LEVEL}" != "debug" ] ; then
        ./fbink -q -y ${PRINT_ROW} -mp "${LOG_MSG}"
    fi
}

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up
logmsg "I" "Enabled loopback interface"

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/kobo-uncaged
KU_LOG=${KU_DIR}/ku_error.log

# Check if we have a ku.toml file. If not, copy it from the default file
if [ ! -f "${KU_DIR}/config/ku.toml" ]; then
    logmsg "I" "No existing config file. Using default."
    cp -f "${KU_DIR}/config/ku.toml.default" "${KU_DIR}/config/ku.toml"
fi

cd ${KU_DIR}

logmsg "I" "Starting Kobo UNCaGED"
$KU_BIN > $KU_LOG
KU_RES=$?

cd -

# Disable the loopback interface when we are done
ip link set lo down
logmsg "I" "Disabled loopback interface"
