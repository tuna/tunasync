# tunasync 上手指南
date: 2016-10-31 00:50:00

[tunasync](https://github.com/tuna/tunasync) 是[清华大学 TUNA 镜像源](https://mirrors.tuna.tsinghua.edu.cn)目前使用的镜像方案。

本文试图在五分钟之内让你搭建一个可以测试的 tunasync 基本功能。

本例中:

 - 只镜像[elvish](https://elvish.io)项目
 - 禁用了https
 - 禁用了cgroup支持

## 获得tunasync

### 二进制包

TODO

### 自行编译

```
$ make
```

## 配置

```
$ mkdir ~/tunasync_demo
$ mkdir /tmp/tunasync
```

`~/tunasync_demo/worker.conf`:

```
[global]
name = "test_worker"
log_dir = "/tmp/tunasync/log/tunasync/{{.Name}}"
mirror_dir = "/tmp/tunasync"
concurrent = 10
interval = 1

[manager]
api_base = "http://localhost:12345"
token = "some_token"
ca_cert = ""

[cgroup]
enable = false
base_path = "/sys/fs/cgroup"
group = "tunasync"

[server]
hostname = "localhost"
listen_addr = "127.0.0.1"
listen_port = 6000
ssl_cert = ""
ssl_key = ""

[include]
include_mirrors = "mirrors/*.conf"
```

`~/tunasync_demo/manager.conf`:

```
debug = false

[server]
addr = "127.0.0.1"
port = 12345
ssl_cert = ""
ssl_key = ""

[files]
db_type = "bolt"
db_file = "/tmp/tunasync/manager.db"
ca_cert = ""
```

### 镜像脚本

```
$ mkdir ~/tunasync_demo/mirrors
$ cat > ~/tunasync_demo/mirrors/elvish.conf < EOF

[[mirrors]]
name = "elvish"
provider = "rsync"
upstream = "rsync://rsync.elvish.io/elvish/"
use_ipv6 = false
EOF
```

### 运行

```
$ tunasync manager --config ~/tunasync_demo/manager.conf
$ tunasync worker --config ~/tunasync_demo/worker.conf
```

本例中，镜像的数据在`/tmp/tunasync/`

## 更进一步

可以参看

```
$ tunasync manager --help
$ tunasync worker --help
```

可以看一下 log 目录
