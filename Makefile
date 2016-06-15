LDFLAGS="-X main.buildstamp=`date -u '+%s'` -X main.githash=`git rev-parse HEAD`"

all: get tunasync tunasynctl

get: 
	go get ./cmd/tunasync
	go get ./cmd/tunasynctl

build:
	mkdir -p build

tunasync: build
	go build -o build/tunasync -ldflags ${LDFLAGS} github.com/tuna/tunasync/cmd/tunasync

tunasynctl: build
	go build -o build/tunasynctl -ldflags ${LDFLAGS} github.com/tuna/tunasync/cmd/tunasynctl
