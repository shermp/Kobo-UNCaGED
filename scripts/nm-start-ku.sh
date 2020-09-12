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

cd ${KU_DIR}

logmsg "I" "Starting Kobo UNCaGED" 3000
$KU_BIN
KU_RES=$?
if [ "$KU_RES" -eq 0 ] ; then
    if [ -f $KU_REPL_MD ] ; then
        logmsg "I" "Updating replacement book filesize(s)" 3000
        sqlite_err=$($SQLITE_BIN $NICKEL_DB ".read ${KU_REPL_MD}" 2>&1 >/dev/null)
        sqlite_res=$?
        if [ $sqlite_res -ne 0 ] ; then 
            logmsg "E" "$sqlite_err" 3000
        fi
    fi
    # Always run library rescan, just in case. Especially to catch book deletion
    logmsg "I" "Running library rescan" 3000
    qndb -s pfmDoneProcessing -m pfmRescanBooksFull
    if [ -f $KU_REPL_MD ] || [ -f $KU_UPDATE_MD ] ; then
        if [ -f $KU_UPDATE_MD ] ; then
            logmsg "I" "Updating metadata" 3000
            sqlite_err=$($SQLITE_BIN $NICKEL_DB ".read ${KU_UPDATE_MD}" 2>&1 >/dev/null)
            sqlite_res=$?
            if [ $sqlite_res -ne 0 ] ; then
                logmsg "E" "$sqlite_err" 3000
            fi
            logmsg "I" "Running library rescan after metadata update" 3000
            qndb -s pfmDoneProcessing -m pfmRescanBooksFull
        fi
        logmsg "I" "Metadata updated" 3000
    fi
    [ -f $KU_REPL_MD ] && rm $KU_REPL_MD
    [ -f $KU_UPDATE_MD ] && rm $KU_UPDATE_MD
elif [ "$KU_RES" -eq 250 ] ; then
    logmsg "I" "Running precautionary library rescan" 3000
    qndb -s pfmDoneProcessing -m pfmRescanBooksFull
fi

cd -

# Disable the loopback interface when we are done
ip link set lo down
logmsg "I" "Disabled loopback interface"

logmsg "I" "Kobo UNCaGED exiting" 3000
