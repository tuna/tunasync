#!/bin/bash
if [ ! -d "$TUNASYNC_WORKING_DIR" ]; then
	echo "Directory not exists, fail"
	exit 1	
fi

function update_homebrew_git() {
	repo_dir="$1"
	cd $repo_dir
	echo "==== SYNC $repo_dir START ===="
	/usr/bin/timeout -s INT 3600 git remote -v update
	echo "==== SYNC $repo_dir DONE ===="
}

update_homebrew_git "$TUNASYNC_WORKING_DIR/homebrew.git"
update_homebrew_git "$TUNASYNC_WORKING_DIR/homebrew-python.git"
update_homebrew_git "$TUNASYNC_WORKING_DIR/homebrew-science.git"
