#!/bin/bash

function sync_repo_ck() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	lftp "${repo_url}/" -e 'mirror -v -P 5 --delete --only-missing --only-newer --no-recursion; bye' || return 1
	wget "${repo_url}/repo-ck.db" -O "repo-ck.db"
}

stat=0
sync_repo_ck "${TUNASYNC_UPSTREAM_URL}/x86_64" "${TUNASYNC_WORKING_DIR}/x86_64" || stat=1
sync_repo_ck "${TUNASYNC_UPSTREAM_URL}/i686" "${TUNASYNC_WORKING_DIR}/i686" || stat=1
exit $stat
