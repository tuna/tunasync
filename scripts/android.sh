#!/bin/bash

function sync_android() {
	cd $TUNASYNC_WORKING_DIR
	/usr/local/bin/android-repo sync
}

sync_android
