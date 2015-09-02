#!/bin/bash

function sync_android() {
	cd $TUNASYNC_WORKING_DIR
	/usr/local/bin/android-repo sync -f
}

function update_repo_config() {
	for repo in $(find $TUNASYNC_WORKING_DIR -type d -not -path "*/.repo/*" -name "*.git")
	do
		cd $repo
		echo $repo
		git config pack.threads 1
	done
}

sync_android
update_repo_config
