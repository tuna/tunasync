FROM --platform=$TARGETPLATFORM golang:1.13-alpine as builder

RUN apk add git musl-dev gcc make --no-cache

ENV GO111MODULE on

ENV GOPROXY https://goproxy.cn

COPY * /mnt/

COPY .* /mnt/

RUN cd /mnt && git checkout twoStageRsync && make all

RUN cd / && git clone https://github.com/tuna/tunasync-scripts.git

FROM --platform=$TARGETPLATFORM alpine:3

RUN apk update && apk add --no-cache rsync wget htop bash python3 py3-pip && \
    mkdir -p /app && mkdir /data && rm -rf /var/cache/apk/* 

RUN wget http://ftp-master.debian.org/ftpsync.tar.gz && tar -vxf *.tar.gz -C / && rm -rf *.tar.gz && cp -rf /distrib/* / && rm -rf /distrib


# 使用变量必须申明
ARG TARGETOS

ARG TARGETARCH

COPY --from=builder /mnt/build-${TARGETOS}-${TARGETARCH}*/* /app/bin/

COPY --from=builder /tunasync-scripts /home/scripts

ENV PATH="${PATH}:/app/bin"

WORKDIR /app

ENTRYPOINT [ "/app/bin/tunasync" ]