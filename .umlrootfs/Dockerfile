FROM debian:buster
RUN apt-get update && apt-get install -y systemd rsh-redone-server ifupdown sudo kmod
RUN echo "host" > /root/.rhosts && \
    chmod 600 /root/.rhosts && \
    /bin/echo -e "auto eth0\niface eth0 inet static\naddress 254.255.255.2/24" > /etc/network/interfaces.d/eth0 && \
    sed -i '/pam_securetty/d' /etc/pam.d/rlogin && \
    cp /usr/share/systemd/tmp.mount  /etc/systemd/system && \
    systemctl enable tmp.mount

RUN echo "deb http://deb.debian.org/debian experimental main" >> /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y make && \
    apt-get install -y -t experimental cgroup-tools
