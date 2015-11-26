#!/bin/bash
set -e

function remove_broken() {
	interval=$1
	interval_file="/tmp/hackage_lastcheck"
	now=`date +%s`

	if [[ -f ${interval_file} ]]; then
		lastcheck=`cat ${interval_file}`
		between=$(echo "${now}-${lastcheck}" | bc)
		[[ $between -lt $interval ]] && echo "skip checking"; return 0
	fi
	echo "start checking"

	mkdir -p "${TUNASYNC_WORKING_DIR}/package"
	cd "${TUNASYNC_WORKING_DIR}/package"

	ls | while read line; do 
		echo -n "$line\t\t"
		tar -tzf $line >/dev/null || (echo "FAIL"; rm $line) && echo "OK"
	done
	
	echo `date +%s` > $interval_file
}

function must_download() {
	src=$1
	dst=$2
	while true; do
		echo "downloading: $name"
		wget "$src" -O "$dst" &>/dev/null || true
		tar -tzf package/$name >/dev/null || rm package/$name && break 
	done
}

function hackage_mirror() {
	local_pklist="/tmp/hackage_local_pklist_$$.list"
	remote_pklist="/tmp/hackage_remote_pklist_$$.list"
	
	cd ${TUNASYNC_WORKING_DIR}
	mkdir -p package

	echo "Downloading index..."
	rm index.tar.gz || true
	axel http://hdiff.luite.com/packages/archive/index.tar.gz -o index.tar.gz > /dev/null
	
	echo "building local package list"
	ls package | sed "s/\.tar\.gz$//" > $local_pklist
	echo "preferred-versions" >> $local_pklist  # ignore preferred-versions
	
	echo "building remote package list"
	tar ztf index.tar.gz | (cut -d/ -f 1,2 2>/dev/null) | sed 's|/|-|' > $remote_pklist
	
	echo "building download list"
	# substract local list from remote list
	comm <(sort $remote_pklist) <(sort $local_pklist) -23 | while read pk; do
		# limit concurrent level
		bgcount=`jobs | wc -l`
		while [[ $bgcount -ge 5 ]]; do
			sleep 0.5
			bgcount=`jobs | wc -l`
		done
		
		name="$pk.tar.gz"
		if [ ! -a package/$name ]; then
			must_download "http://hackage.haskell.org/package/$pk/$name" "package/$name" &
		else
			echo "skip existed: $name"
		fi
	done
	
	# delete redundanty files
	comm <(sort $remote_pklist) <(sort $local_pklist) -13 | while read pk; do
		name="$pk.tar.gz"
		echo "deleting ${name}"
		rm "package/$name"
	done

	cp index.tar.gz 00-index.tar.gz
}

function cleanup () {
	echo "cleaning up"
	[[ ! -z $local_pklist ]] && (rm $local_pklist $remote_pklist ; true)
}

trap cleanup EXIT
remove_broken 86400
hackage_mirror 

# vim: ts=4 sts=4 sw=4
