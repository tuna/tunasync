#!/bin/bash

REPO=${REPO:-"/usr/local/bin/repo"}

function repo_init() {
	mkdir -p $TUNASYNC_WORKING_DIR
	cd $TUNASYNC_WORKING_DIR
	$REPO init -u https://android.googlesource.com/mirror/manifest --mirror
}

function repo_sync() {
	cd $TUNASYNC_WORKING_DIR
	$REPO sync -f
}

if [ ! -d "$TUNASYNC_WORKING_DIR/git-repo.git" ]; then
	echo "Initializing AOSP mirror"
	repo_init
fi

repo_sync
