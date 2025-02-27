module github.com/tuna/tunasync

go 1.23.0

toolchain go1.23.5

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/alicebob/miniredis v2.5.0+incompatible
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/boltdb/bolt v1.3.1
	github.com/codeskyblue/go-sh v0.0.0-20200712050446-30169cf553fe
	github.com/containerd/cgroups/v3 v3.0.5
	github.com/dennwc/btrfs v0.0.0-20241002142654-12ae127e0bf6
	github.com/dgraph-io/badger/v2 v2.2007.4
	github.com/docker/go-units v0.5.0
	github.com/gin-gonic/gin v1.10.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/imdario/mergo v0.3.16
	github.com/moby/moby v28.0.1+incompatible
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.7.0
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46
	github.com/smartystreets/goconvey v1.6.4
	github.com/syndtr/goleveldb v1.0.0
	github.com/urfave/cli v1.22.16
	golang.org/x/sys v0.30.0
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473
)

replace github.com/boltdb/bolt v1.3.1 => go.etcd.io/bbolt v1.3.11

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/bytedance/sonic v1.12.9 // indirect
	github.com/bytedance/sonic/loader v0.2.3 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.17.3 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/codegangsta/inject v0.0.0-20150114235600-33e0aa1cb7c0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/dennwc/ioctl v1.0.0 // indirect
	github.com/dgraph-io/ristretto v0.2.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20240924180020-3414d57e47da // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sse v1.0.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.25.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.8.2 // indirect
	github.com/google/pprof v0.0.0-20250208200701-d0013a598941 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/smartystreets/assertions v1.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/yuin/gopher-lua v0.0.0-20191220021717-ab39c6098bdb // indirect
	golang.org/x/arch v0.14.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
