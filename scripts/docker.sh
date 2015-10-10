#!/bin/bash

function sync_docker() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	# lftp "${repo_url}/" -e "mirror --verbose --exclude-glob='*/SRPMS/*' -P 5 --delete --only-newer; bye"
	# lftp "${repo_url}/" -e "mirror --verbose  -P 5 --delete --only-newer; bye"
	wget --mirror --convert-links --no-parent --no-host-directories $repo_url 
	find . -type f -iname "*.1" -exec rm {} \;
}

sync_docker "http://apt.dockerproject.org/" "${TUNASYNC_WORKING_DIR}/apt"
sync_docker "http://yum.dockerproject.org/" "${TUNASYNC_WORKING_DIR}/yum"
