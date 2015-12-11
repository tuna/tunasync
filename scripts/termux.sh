#!/bin/bash
set -e

_here=`dirname $(realpath $0)`
. ${_here}/helpers/apt-download
[ -z "${LOADED_APT_DOWNLOAD}" ] && (echo "failed to load apt-download"; exit 1)

BASE_PATH="${TUNASYNC_WORKING_DIR}"

base_url="http://apt.termux.com"
ARCHES=("aarch64" "all" "arm" "i686")
for arch in ${ARCHES[@]}; do
	echo "start syncing: ${arch}"
	apt-download-binary "${base_url}" "stable" "main" "${arch}" "${BASE_PATH}" || true
done
echo "finished"
