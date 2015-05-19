#!/bin/bash
SYNC_FILES="$TUNASYNC_WORKING_DIR"
# SYNC_FILES="/srv/mirror_disk/ubuntu/_working/"
#LOG_FILE="$TUNASYNC_LOG_FILE"

# [ -f $SYNC_LOCK ] && exit 1
# touch $SYNC_LOCK


echo ">> Starting sync on $(date --rfc-3339=seconds)"

arch="i386,amd64"
sections="main,main/debian-installer,multiverse,multiverse/debian-installer,restricted,restricted/debian-installer,universe,universe/debian-installer"
dists="precise,precise-backports,precise-proposed,precise-updates,precise-security,trusty,trusty-backports,trusty-proposed,trusty-updates,trusty-security"
server="$1"
inPath="/ubuntu"
proto="rsync"
outpath="$SYNC_FILES"
rsyncOpt='-6 -aIL --partial'

debmirror -h $server --no-check-gpg -a $arch -s $sections -d $dists -r $inPath -e $proto --rsync-options "$rsyncOpt" --verbose $outpath

date --rfc-3339=seconds > "$SYNC_FILES/lastsync"
echo ">> Finished sync on $(date --rfc-3339=seconds)"

# rm -f "$SYNC_LOCK"
exit 0
