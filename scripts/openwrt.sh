#!/bin/bash

function sync_openwrt() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	lftp "${repo_url}/" -e "mirror --verbose -P 5 --delete --only-newer; bye"
}

sync_openwrt "http://downloads.openwrt.org/chaos_calmer/15.05/" "${TUNASYNC_WORKING_DIR}/chaos_calmer/15.05"
sync_openwrt "http://downloads.openwrt.org/snapshots/trunk/" "${TUNASYNC_WORKING_DIR}/snapshots/trunk"
