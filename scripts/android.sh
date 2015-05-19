#!/bin/bash

function sync_android() {
	cd $TUNASYNC_WORKING_DIR
	/usr/local/bin/android-repo sync -f
}

function update_server_info() {
	for repo in $(find $TUNASYNC_WORKING_DIR -type d -not -path "*/.repo/*" -name "*.git")
	do
		cd $repo
		git update-server-info
	done
}

sync_android
