LDFLAGS="-X main.buildstamp=`date -u '+%s'` -X main.githash=`git rev-parse HEAD`"
ARCH ?= linux-amd64
ARCH_LIST = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(ARCH_LIST))
GOARCH = $(word 2, $(ARCH_LIST))
BUILDBIN = tunasync tunasynctl

all: $(BUILDBIN)

build-$(ARCH):
	mkdir -p $@

$(BUILDBIN): % : build-$(ARCH) build-$(ARCH)/%

$(BUILDBIN:%=build-$(ARCH)/%) : build-$(ARCH)/% : cmd/%
	GOOS=$(GOOS) GOARCH=$(GOARCH) go get ./$<
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $@ -ldflags ${LDFLAGS} github.com/tuna/tunasync/$<

test:
# see: https://stackoverflow.com/questions/79780882/go-no-such-tool-covdata-in-go-1-25
	GOTOOLCHAIN=go1.26.2+auto go test -v -covermode=count -coverprofile=profile.gcov ./...

build-test-worker:
	CGO_ENABLED=0 go test -c -covermode=count github.com/tuna/tunasync/worker

clean:
	rm -rf build-$(ARCH)

.PHONY: all test $(BUILDBIN) build-test-worker clean
