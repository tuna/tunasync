# tunasync 上手指南

date: 2016-10-31 00:50:00

[tunasync](https://github.com/tuna/tunasync) 是[清华大学 TUNA 镜像源](https://mirrors.tuna.tsinghua.edu.cn)目前使用的镜像方案。

本文试图在五分钟之内让你搭建一个可以测试的 tunasync 基本功能。

本例中:

- 只镜像[elvish](https://elv.sh)项目
- 禁用了https
- 禁用了cgroup支持

## 获得tunasync

### 二进制包

到 [Github Releases](https://github.com/tuna/tunasync/releases/latest) 下载 `tunasync-linux-amd64-bin.tar.gz` 即可。

### 自行编译

```shell
> make
```

## 配置

```shell
> mkdir ~/tunasync_demo
> mkdir /tmp/tunasync
```

编辑 `~/tunasync_demo/worker.conf`:

```conf
[global]
name = "test_worker"
log_dir = "/tmp/tunasync/log/tunasync/{{.Name}}"
mirror_dir = "/tmp/tunasync"
concurrent = 10
interval = 1

[manager]
api_base = "http://localhost:12345"
token = ""
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

[[mirrors]]
name = "elvish"
provider = "rsync"
upstream = "rsync://rsync.elv.sh/elvish/"
use_ipv6 = false
```

编辑 `~/tunasync_demo/manager.conf`:

```conf
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

除了 bolt 以外，还支持 badger、leveldb 和 redis 的数据库后端。对于 badger 和 leveldb，只需要修改 db_type。如果使用 redis 作为数据库后端，把 db_type 改为 redis，并把下面的 db_file 设为 redis 服务器的地址： `redis://user:password@host:port/db_number`。

### 运行

```shell
> tunasync manager --config ~/tunasync_demo/manager.conf
> tunasync worker --config ~/tunasync_demo/worker.conf
```

本例中，镜像的数据在 `/tmp/tunasync/`。

### 控制

查看同步状态

```shell
> tunasynctl list -p 12345 --all
```

tunasynctl 也支持配置文件。配置文件可以放在 `/etc/tunasync/ctl.conf` 或者 `~/.config/tunasync/ctl.conf` 两个位置，后者可以覆盖前者的配置值。

配置文件内容为：

```conf
manager_addr = "127.0.0.1"
manager_port = 12345
ca_cert = ""
```

### 安全

worker 和 manager 之间用 http(s) 通信，如果你 worker 和 manager 都是在本机，那么没必要使用 https。此时 manager 就不指定 `ssl_key` 和 `ssl_cert`，留空；worker 的 `ca_cert` 留空，`api_base` 以 `http://` 开头。

如果需要加密的通信，manager 需要指定 `ssl_key` 和 `ssl_cert`，worker 要指定 `ca_cert`，并且 `api_base` 应该是 `https://` 开头。

## 更进一步

可以参看

```shell
> tunasync manager --help
> tunasync worker --help
```

可以看一下 log 目录

一些 worker 配置文件示例 [workers.conf](workers.conf)。

你可能会用到的操作 [tips.md](tips.md)。
