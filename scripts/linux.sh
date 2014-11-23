#!/bin/bash
if [ ! -d "$TUNASYNC_WORKING_DIR" ]; then
	echo "Directory not exists, fail"
	exit 1	
fi

function update_linux_git() {
	cd $TUNASYNC_WORKING_DIR
	/usr/bin/timeout -s INT 3600 git remote -v update
}

update_linux_git
