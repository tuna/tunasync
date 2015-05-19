#!/bin/bash

function sync_repo_ck() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	lftp "${repo_url}/" -e 'mirror -v -P 5 --delete --only-missing --only-newer --no-recursion; bye'
	wget "${repo_url}/repo-ck.db" -O "repo-ck.db"
	wget "${repo_url}/repo-ck.files" -O "repo-ck.files"
}

UPSTREAM="http://repo-ck.com"

sync_repo_ck "${UPSTREAM}/x86_64" "${TUNASYNC_WORKING_DIR}/x86_64"
sync_repo_ck "${UPSTREAM}/i686" "${TUNASYNC_WORKING_DIR}/i686"
