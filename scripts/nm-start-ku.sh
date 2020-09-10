#!/bin/sh

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/kobo-uncaged
SQLITE_BIN=${KU_DIR}/bin/sqlite3
NICKEL_DB=/mnt/onboard/.kobo/KoboReader.sqlite
FBINK_BIN=${KU_DIR}/bin/fbink
KU_LOG=${KU_DIR}/ku_error.log

KU_REPL_MD=${KU_DIR}/replace-book.sql
KU_UPDATE_MD=${KU_DIR}/updated-md.sql

# Inport logmsg function
. ${KU_DIR}/scripts/ku-lib.sh

# Ensure before beginning that any sql files from prior runs are removed
[ -f $KU_REPL_MD ] && rm $KU_REPL_MD
[ -f $KU_UPDATE_MD ] && rm $KU_UPDATE_MD

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up
logmsg "I" "Enabled loopback interface"

# Check if we have a ku.toml file. If not, copy it from the default file
if [ ! -f "${KU_DIR}/config/ku.toml" ]; then
    logmsg "I" "No existing config file. Using default."
    cp -f "${KU_DIR}/config/ku.toml.default" "${KU_DIR}/config/ku.toml"
fi

# # Use NickelDBus to connect to Wifi
# WIFI_OUTPUT=$(qndb -s wmNetworkConnected -s wmNetworkFailedToConnect -t 60000 -m wfmConnectWireless)
# if ! $? ; then
#     qndb -m mwcToast "Connecting to Wifi errored or timed out"
#     logmsg "E" "$WIFI_OUTPUT"
#     exit 1
# elif [ "$WIFI_OUTPUT" = "wmNetworkFailedToConnect" ] ; then
#     qndb -m mwcToast "Could not connect to WiFi"
#     logmsg "E" "Could not connect to WiFi"
#     exit 1
# fi

cd ${KU_DIR}

logmsg "I" "Starting Kobo UNCaGED"
$KU_BIN > $KU_LOG
KU_RES=$?

if [ -f $KU_REPL_MD ] ; then
    logmsg "I" "Updating replacement book filesize(s)"
    qndb -m mwcToast 3000 "Updating replacement book filesize(s)"
    sqlite_err=$($SQLITE_BIN $NICKEL_DB ".read ${KU_REPL_MD}" 2>&1 >/dev/null)
    sqlite_res=$?
    if [ $sqlite_res -ne 0 ] ; then 
        logmsg "E" "$sqlite_err"
        qndb -m mwcToast 3000 "SQL error updating filesize"
    fi
fi
if [ -f $KU_REPL_MD ] || [ -f $KU_UPDATE_MD ] ; then
    logmsg "I" "Running book rescan"
    qndb -m mwcToast 3000 "Running book rescan"
    qndb -s pfmDoneProcessing -m pfmRescanBooksFull
    if [ -f $KU_UPDATE_MD ] ; then
        logmsg "I" "Updating metadata"
        qndb -m mwcToast 3000 "Updating metadata"
        sqlite_err=$($SQLITE_BIN $NICKEL_DB ".read ${KU_UPDATE_MD}" 2>&1 >/dev/null)
        sqlite_res=$?
        if [ $sqlite_res -ne 0 ] ; then
            logmsg "E" "$sqlite_err"
            qndb -m mwcToast 3000 "SQL error updating metadata"
        fi
        logmsg "I" "Running book rescan after metadata update"
        qndb -m mwcToast 3000 "Running book rescan after metadata update"
        qndb -s pfmDoneProcessing -m pfmRescanBooksFull
    fi
    logmsg "I" "Metadata updated"
    qndb -m mwcToast 3000 "Metadata updated"
fi
[ -f $KU_REPL_MD ] && rm $KU_REPL_MD
[ -f $KU_UPDATE_MD ] && rm $KU_UPDATE_MD

cd -

# Disable the loopback interface when we are done
ip link set lo down
logmsg "I" "Disabled loopback interface"
