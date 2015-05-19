#!/bin/bash

function sync_repo_ck() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	# lftp "${repo_url}/" -e "mirror --verbose --log=${TUNASYNC_LOG_FILE} --exclude-glob='*/SRPMS/*' -P 5 --delete --only-newer; bye"
	lftp "${repo_url}/" -e "mirror --verbose  -P 5 --delete --only-newer; bye"
}

sync_repo_ck "https://deb.nodesource.com/node" "${TUNASYNC_WORKING_DIR}/deb"
sync_repo_ck "https://rpm.nodesource.com/pub" "${TUNASYNC_WORKING_DIR}/rpm"
