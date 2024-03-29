#!/bin/sh

KU_DIR=/mnt/onboard/.adds/kobo-uncaged
KU_BIN=${KU_DIR}/bin/ku
SQLITE_BIN=${KU_DIR}/bin/sqlite3
NICKEL_DB=/mnt/onboard/.kobo/KoboReader.sqlite

KU_REPL_MD=${KU_DIR}/replace-book.sql
KU_UPDATE_MD=${KU_DIR}/updated-md.sql

# Delete previous log file if it exists
[ -f "$KU_LOGFILE" ] && rm "$KU_LOGFILE"

# Inport logmsg function
. ${KU_DIR}/scripts/ku-lib.sh

delete_dir() {
    del_dir="$1"
    # Protect against deleting anything outside of the ku directory
    case "$del_dir" in
        "/mnt/onboard/.adds/kobo-uncaged/"*)
            if [ -d "$del_dir" ] ; then
                logmsg "I" "Directory ${del_dir} exists. Deleting..."
                rm -rf "$del_dir"
            fi
            ;;
        *) logmsg "W" "${del_dir} not in ${KU_DIR}. Not deleting" ;;
    esac
}

# Cleanup 'static' and 'template' directories from older versions
delete_dir "${KU_DIR}/static"
delete_dir "${KU_DIR}/templates"

call_sqlite() {
    sql_file="$1"
    sqlite_err=$($SQLITE_BIN $NICKEL_DB 2>&1 >/dev/null <<EOF
.timeout 3000
.read ${sql_file}
EOF
)
    sqlite_res=$?
    if [ $sqlite_res -ne 0 ] ; then 
        logmsg "E" "$sqlite_err" 5000
    fi
}

# In case we aren't launched with NickelMenu, check that NickelDBus is
# installed and available before continuing
if ! ndb_installed ; then
    logmsg "C" "NickelDBus not installed. Aborting."
    exit 1
fi

# Ensure before beginning that any sql files from prior runs are removed
[ -f $KU_REPL_MD ] && rm $KU_REPL_MD
[ -f $KU_UPDATE_MD ] && rm $KU_UPDATE_MD

# For some reason, kobo's don't enable the loopback network interface
# We take care of it here
ip link set lo up
logmsg "I" "Enabled loopback interface"

# The zip file may not contain the 'config' directory, so ensure it is created
[ -d "${KU_DIR}/config" ] || mkdir "${KU_DIR}/config"

cd ${KU_DIR}

logmsg "I" "Starting Kobo UNCaGED" 1000
$KU_BIN
KU_RES=$?
if [ "$KU_RES" -eq 0 ] ; then
    if [ -f $KU_REPL_MD ] ; then
        logmsg "I" "Updating replacement book filesize(s)" 1000
        call_sqlite "$KU_REPL_MD"
    fi
    # Always run library rescan, just in case. Especially to catch book deletion
    logmsg "I" "Running library rescan" 1000
    qndb -s pfmDoneProcessing -m pfmRescanBooksFull
    if [ -f $KU_REPL_MD ] || [ -f $KU_UPDATE_MD ] ; then
        if [ -f $KU_UPDATE_MD ] ; then
            logmsg "I" "Updating metadata" 1000
            call_sqlite "$KU_UPDATE_MD"
            logmsg "I" "Running library rescan after metadata update" 1000
            qndb -s pfmDoneProcessing -m pfmRescanBooksFull
        fi
        logmsg "I" "Metadata updated"
    fi
    [ -f $KU_REPL_MD ] && rm $KU_REPL_MD
    [ -f $KU_UPDATE_MD ] && rm $KU_UPDATE_MD
elif [ "$KU_RES" -eq 250 ] ; then
    logmsg "I" "Running precautionary library rescan" 1000
    qndb -s pfmDoneProcessing -m pfmRescanBooksFull
fi

cd -

# Disable the loopback interface when we are done
ip link set lo down
logmsg "I" "Disabled loopback interface"

# End with a confirmation dialog. Saves the user having to keep an eagle eye on the screen
# to know when KU has finished.
qndb -m dlgConfirmAccept "Kobo UNCaGED" "Finished!" "Continue"
