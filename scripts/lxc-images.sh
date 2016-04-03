#!/bin/bash

function sync_lxc_images() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir

	# lftp "${repo_url}/" -e "mirror --verbose --log=${TUNASYNC_LOG_FILE} --exclude-glob='*/SRPMS/*' -P 5 --delete --only-newer; bye"
	lftp "${repo_url}/" -e "mirror --verbose -P 5 --delete --only-newer; bye"
}


sync_lxc_images "http://images.linuxcontainers.org/images" "${TUNASYNC_WORKING_DIR}/images"
sync_lxc_images "http://images.linuxcontainers.org/meta" "${TUNASYNC_WORKING_DIR}/meta"
