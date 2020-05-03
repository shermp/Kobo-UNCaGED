#!/bin/sh

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/kobo-uncaged
FBINK_BIN=${KU_DIR}/bin/fbink
KU_LOG=${KU_DIR}/ku_error.log

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

    # Print warnings and errors to screen
    if [ "${LOG_LEVEL}" != "debug" ] || [ "${LOG_LEVEL}" != "info" ] ; then
        $FBINK_BIN -q -y ${PRINT_ROW} -mp "${LOG_MSG}"
    fi
}

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up
logmsg "I" "Enabled loopback interface"

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
