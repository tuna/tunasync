#!/bin/bash

function remove_broken() {
	working_dir=$1
	cd $working_dir
	mkdir -p package
	filelist="/tmp/hackage_filelist_$$.txt"
	brokenlist="/tmp/hackage_brokenlist_$$.txt"
	ls > ${filelist}
	touch $brokenlist
	while read line; do echo $line ; tar -tzf $line >/dev/null || echo $line >>$brokenlist; done <$filelist
	cat $brokenlist | xargs rm

	rm $brokenlist
}

function hackage_mirror() {
	working_dir=$1
	cd $working_dir
	# echo "Cleaning up..."
	# rm 00-index.tar.gz
	mkdir -p package
	echo "Downloading index..."
	rm index.tar.gz
	axel http://hdiff.luite.com/packages/archive/index.tar.gz -o index.tar.gz
	for splitpk in `tar ztf index.tar.gz | cut -d/ -f 1,2 2>/dev/null`; do
		pk=`echo $splitpk | sed 's|/|-|'`
		name=$pk.tar.gz
		if [[ ! -a package/$name ]]; then
			axel http://hackage.haskell.org/package/$pk/$name -o package/$name
		fi
	done
	rm package/preferred-versions.tar.gz
	cp index.tar.gz 00-index.tar.gz
}

remove_broken "${TUNASYNC_WORKING_DIR}/"
hackage_mirror "${TUNASYNC_WORKING_DIR}/"
