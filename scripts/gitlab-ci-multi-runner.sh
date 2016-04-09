#!/bin/bash
# reqires: wget, yum-utils
set -e
set -o pipefail

_here=`dirname $(realpath $0)`
. ${_here}/helpers/apt-download

[ -z "${LOADED_APT_DOWNLOAD}" ] && (echo "failed to load apt-download"; exit 1)

BASE_PATH="${TUNASYNC_WORKING_DIR}"

YUM_PATH="${BASE_PATH}/yum"

UBUNTU_VERSIONS=("trusty" "xenial")
DEBIAN_VERSIONS=("wheezy" "jessie" "stretch")
UBUNTU_PATH="${BASE_PATH}/ubuntu/"
DEBIAN_PATH="${BASE_PATH}/debian/"

mkdir -p $UBUNTU_PATH $DEBIAN_PATH $YUM_PATH

cache_dir="/tmp/yum-gitlab-runner-cache/"
cfg="/tmp/gitlab-runner-yum.conf"
cat <<EOF > ${cfg}
[main]
keepcache=0

[el6]
name=gitlab-ci-multi-runner-el6
baseurl=https://packages.gitlab.com/runner/gitlab-ci-multi-runner/el/6/x86_64
repo_gpgcheck=0
gpgcheck=0
enabled=1
gpgkey=https://packages.gitlab.com/gpg.key
sslverify=0

[el7]
name=gitlab-ci-multi-runner-el7
baseurl=https://packages.gitlab.com/runner/gitlab-ci-multi-runner/el/7/x86_64
repo_gpgcheck=0
gpgcheck=0
enabled=1
gpgkey=https://packages.gitlab.com/gpg.key
sslverify=0
EOF

reposync -c $cfg -d -p ${YUM_PATH}  -e $cache_dir
[ ! -d ${YUM_PATH}/el6 ] && mkdir -p ${YUM_PATH}/el6
[ ! -d ${YUM_PATH}/el7 ] && mkdir -p ${YUM_PATH}/el7
createrepo --update -v -c $cache_dir -o ${YUM_PATH}/el6 ${YUM_PATH}/el6
createrepo --update -v -c $cache_dir -o ${YUM_PATH}/el7 ${YUM_PATH}/el7
rm $cfg

base_url="https://packages.gitlab.com/runner/gitlab-ci-multi-runner/ubuntu"
for version in ${UBUNTU_VERSIONS[@]}; do
	apt-download-binary ${base_url} "$version" "main" "amd64" "${UBUNTU_PATH}" || true
	apt-download-binary ${base_url} "$version" "main" "i386" "${UBUNTU_PATH}" || true
done
echo "Ubuntu finished"

base_url="https://packages.gitlab.com/runner/gitlab-ci-multi-runner/debian"
for version in ${DEBIAN_VERSIONS[@]}; do
	apt-download-binary ${base_url} "$version" "main" "amd64" "${DEBIAN_PATH}" || true
	apt-download-binary ${base_url} "$version" "main" "i386" "${DEBIAN_PATH}" || true
done
echo "Debian finished"


# vim: ts=4 sts=4 sw=4
