#!/bin/bash
# requires: wget, lftp, jq
#

set -e
set -o pipefail

CONDA_REPO_BASE=${CONDA_REPO_BASE:-"http://repo.continuum.io"}
LOCAL_DIR_BASE="${TUNASYNC_WORKING_DIR}/pkgs"
TMP_DIR=$(mktemp -d)

CONDA_REPOS=("free" "r" "mro" "pro")
CONDA_ARCHES=("linux-64" "linux-32" "linux-armv6l" "linux-armv7l" "linux-ppc64le" "osx-64" "osx-32" "win-64" "win-32")

function check-and-download() {
	remote_file=$1
	local_file=$2
	wget -q --spider ${remote_file}
	if [ $? -eq 0 ]; then
		echo "downloading ${remote_file}"
		wget -q -N -O ${local_file} ${remote_file}
		return
	fi
	return 1
}

function cleanup () {
	echo "cleaning up"
	[ -d ${TMP_DIR} ] && {
		[ -f ${TMP_DIR}/repodata.json ] && rm ${TMP_DIR}/repodata.json
		[ -f ${TMP_DIR}/repodata.json.bz2 ] && rm ${TMP_DIR}/repodata.json.bz2
		rmdir ${TMP_DIR}
	}
}

trap cleanup EXIT

echo ${TMP_DIR}

for repo in ${CONDA_REPOS[@]}; do
	for arch in ${CONDA_ARCHES[@]}; do
		PKG_REPO_BASE="${CONDA_REPO_BASE}/pkgs/$repo/$arch"
		repodata_url="${PKG_REPO_BASE}/repodata.json"
		bz2_repodata_url="${PKG_REPO_BASE}/repodata.json.bz2"
		LOCAL_DIR="${LOCAL_DIR_BASE}/$repo/$arch"
		[ ! -d ${LOCAL_DIR} ] && mkdir -p ${LOCAL_DIR}
		tmp_repodata="${TMP_DIR}/repodata.json"
		tmp_bz2_repodata="${TMP_DIR}/repodata.json.bz2"

		check-and-download ${repodata_url} ${tmp_repodata}
		check-and-download ${bz2_repodata_url} ${tmp_bz2_repodata}

		jq_cmd='.packages | to_entries[] | [.key, .value.size, .value.md5] | map(tostring) | join(" ")'
		bzip2 -c -d ${tmp_bz2_repodata} | jq -r "${jq_cmd}" | while read line;
		do
			read -a tokens <<< $line
			pkgfile=${tokens[0]}
			pkgsize=${tokens[1]}
			pkgmd5=${tokens[2]}
			
			pkg_url="${PKG_REPO_BASE}/${pkgfile}"
			dest_file="${LOCAL_DIR}/${pkgfile}"
			
			declare downloaded=false
			if [ -f ${dest_file} ]; then
				rsize=`stat -c "%s" ${dest_file}`
				if [ ${rsize} -eq ${pkgsize} ]; then
					downloaded=true
					echo "Skipping ${pkgfile}, size ${pkgsize}"
				fi
			fi
			while [ $downloaded != true ]; do
				echo "downloading ${pkg_url}"
				wget -q -O ${dest_file} ${pkg_url} && {
					# two space for md5sum check format
					echo "${pkgmd5}  ${dest_file}" | md5sum -c - && downloaded=true 
				}
			done
		done
		
		mv -f "${TMP_DIR}/repodata.json" "${LOCAL_DIR}/repodata.json"
		mv -f "${TMP_DIR}/repodata.json.bz2" "${LOCAL_DIR}/repodata.json.bz2"
	done
done

function sync_installer() {
	repo_url="$1"
	repo_dir="$2"

	[ ! -d "$repo_dir" ] && mkdir -p "$repo_dir"
	cd $repo_dir
	lftp "${repo_url}/" -e "mirror --verbose -P 5; bye"
}

sync_installer "${CONDA_REPO_BASE}/archive/" "${TUNASYNC_WORKING_DIR}/archive/"
sync_installer "${CONDA_REPO_BASE}/miniconda/" "${TUNASYNC_WORKING_DIR}/miniconda/"
