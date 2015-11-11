#!/bin/bash
set -e

BASE_PATH="${TUNASYNC_WORKING_DIR}"


APT_PATH="${BASE_PATH}/apt"
YUM_PATH="${BASE_PATH}/yum"

UBUNTU_VERSIONS=("trusty" "wily")
DEBIAN_VERSIONS=("wheezy" "jessie" "stretch")

mkdir -p $UBUNTU_PATH $DEBIAN_PATH $YUM_PATH


cfg="/tmp/gitlab-ce-yum.conf"
cat <<EOF > ${cfg}
[el6]
name=el6
baseurl=https://packages.gitlab.com/gitlab/gitlab-ce/el/6/x86_64
repo_gpgcheck=0
gpgcheck=0
enabled=1
gpgkey=https://packages.gitlab.com/gpg.key
sslverify=0

[el7]
name=el7
baseurl=https://packages.gitlab.com/gitlab/gitlab-ce/el/7/x86_64
repo_gpgcheck=0
gpgcheck=0
enabled=1
gpgkey=https://packages.gitlab.com/gpg.key
sslverify=0
EOF

reposync -c $cfg -d -p ${YUM_PATH}
createrepo -o ${YUM_PATH}/el6 ${YUM_PATH}/el6
createrepo -o ${YUM_PATH}/el7 ${YUM_PATH}/el7
rm $cfg


cfg="/tmp/gitlab-ce-ubuntu.list"
cat << EOF > ${cfg}
set mirror_path ${APT_PATH}
set nthreds 5
set _tilde 0

EOF
for version in ${UBUNTU_VERSIONS[@]}; do
		echo "deb https://packages.gitlab.com/gitlab/gitlab-ce/ubuntu/ $version main" >> $cfg
		echo "deb-i386 https://packages.gitlab.com/gitlab/gitlab-ce/ubuntu/ $version main" >> $cfg
		echo "deb-amd64 https://packages.gitlab.com/gitlab/gitlab-ce/ubuntu/ $version main" >> $cfg
		echo "deb-src https://packages.gitlab.com/gitlab/gitlab-ce/ubuntu/ $version main" >> $cfg
done

apt-mirror $cfg
rm $cfg


cfg="/tmp/gitlab-ce-debian.list"
cat << EOF > ${cfg}
set mirror_path ${APT_PATH}
set nthreds 5
set _tilde 0

EOF
for version in ${DEBIAN_VERSIONS[@]}; do
		echo "deb https://packages.gitlab.com/gitlab/gitlab-ce/debian/ $version main" >> $cfg
		echo "deb-i386 https://packages.gitlab.com/gitlab/gitlab-ce/debian/ $version main" >> $cfg
		echo "deb-amd64 https://packages.gitlab.com/gitlab/gitlab-ce/debian/ $version main" >> $cfg
		echo "deb-src https://packages.gitlab.com/gitlab/gitlab-ce/debian/ $version main" >> $cfg
done

apt-mirror $cfg
rm $cfg

# vim: ts=4 sts=4 sw=4
