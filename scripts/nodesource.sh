#!/bin/bash

function sync_nodesource() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	# lftp "${repo_url}/" -e "mirror --verbose --exclude-glob='*/SRPMS/*' -P 5 --delete --only-newer; bye"
	lftp "${repo_url}/" -e "mirror --verbose  -P 5 --delete --only-newer; bye"
}

sync_nodesource "https://deb.nodesource.com/node" "${TUNASYNC_WORKING_DIR}/deb"
sync_nodesource "https://deb.nodesource.com/node_0.12" "${TUNASYNC_WORKING_DIR}/deb_0.12"
sync_nodesource "https://deb.nodesource.com/node_4.x" "${TUNASYNC_WORKING_DIR}/deb_4.x"
sync_nodesource "https://rpm.nodesource.com/pub" "${TUNASYNC_WORKING_DIR}/rpm"
sync_nodesource "https://rpm.nodesource.com/pub_0.12" "${TUNASYNC_WORKING_DIR}/rpm_0.12"
sync_nodesource "https://rpm.nodesource.com/pub_4.x" "${TUNASYNC_WORKING_DIR}/rpm_4.x"
