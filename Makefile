LDFLAGS="-X main.buildstamp=`date -u '+%s'` -X main.githash=`git rev-parse HEAD`"

all: get tunasync tunasynctl

travis: get tunasync tunasynctl travis-package

get: 
	go get ./cmd/tunasync
	go get ./cmd/tunasynctl

build:
	mkdir -p build

tunasync: build
	go build -o build/tunasync -ldflags ${LDFLAGS} github.com/tuna/tunasync/cmd/tunasync

tunasynctl: build
	go build -o build/tunasynctl -ldflags ${LDFLAGS} github.com/tuna/tunasync/cmd/tunasynctl

travis-package: tunasync tunasynctl
	tar zcf build/tunasync-linux-bin.tar.gz -C build tunasync tunasynctl

test:
	go test -v -covermode=count -coverprofile=profile.cov ./...
