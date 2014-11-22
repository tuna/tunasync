#!/bin/bash
if [ ! -d "$TUNASYNC_WORKING_DIR" ]; then
	echo "Directory not exists, fail"
	exit 1	
fi

function update_homebrew_git() {
	cd $TUNASYNC_WORKING_DIR
	git remote -v update
}

update_homebrew_git
