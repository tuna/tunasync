FROM debian:trixie
# Prevent from bumping the base image version to later than trixie,
# because systemd in later versions of Debian removed the support for legacy
# cgroup hierarchy

RUN apt-get update && apt-get install -y systemd openssh-server ifupdown sudo kmod cgroup-tools systemd-sysv
RUN \
    /bin/echo -e "auto eth0\niface eth0 inet static\naddress 254.255.255.2/24" > /etc/network/interfaces.d/eth0
