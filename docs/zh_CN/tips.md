## 删除某worker的某镜像

先确定已经给tunasynctl写好config文件：`~/.config/tunasync/ctl.conf`

```toml
manager_addr = "127.0.0.1"
manager_port = 12345
ca_cert = ""
```

接着

```shell
$ tunasynctl disable -w <worker_id> <mirror_name>
$ tunasynctl flush
```


## 热重载 `worker.conf`

```shell
$ tunasynctl reload -w <worker_id>
```

e.g. 删除 `test_worker` 的 `elvish` 镜像：

1. 删除存放镜像的文件夹

2. 删除 `worker.conf` 中对应的 `mirror` 段落

3. 接着操作：

```shell
$ tunasynctl reload -w test_worker
$ tunasynctl disable -w test_worker elvish
$ tunasynctl flush
```

4. （可选）最后删除日志文件夹里的日志


## 删除worker

```shell
$ tunasynctl rm-worker -w <worker_id>
```

e.g.

```shell
$ tunasynctl rm-worker -w test_worker
```


## 更新镜像的大小

```shell
$ tunasynctl set-size -w <worker_id> <mirror_name> <size>
```

其中，末尾的 <size> 参数，由操作者设定，或由某定时脚本生成

由于 `du -s` 比较耗时，故镜像大小可直接由rsync的日志文件读出


## Btrfs 文件系统快照

如果镜像文件存放在以 Btrfs 为文件系统的分区中，可启用由 Btrfs 提供的快照 (Snapshot) 功能。对于每一个镜像，tunasync 在每次成功同步后更新其快照。

在 `worker.conf` 中添加如下配置，即可启用 Btrfs 快照功能：

```toml
[btrfs_snapshot]
enable = true
snapshot_path = "/path/to/snapshot/directory"
```

其中 `snapshot_path` 为快照所在目录。如将其作为发布版本，则镜像同步过程对于镜像站用户而言具有原子性。如此可避免用户接收到仍处于“中间态”的（未完成同步的）文件。

也可以在 `[[mirrors]]` 中为特定镜像单独指定快照路径，如：

```toml
[[mirrors]]
name = "elvish"
provider = "rsync"
upstream = "rsync://rsync.elv.sh/elvish/"
interval = 1440
snapshot_path = "/data/publish/elvish"
```

**提示：** 

若运行 tunasync 的用户无 root 权限，请确保该用户对镜像同步目录和快照目录均具有写和执行权限，并使用 [`user_subvol_rm_allowed` 选项](https://btrfs.wiki.kernel.org/index.php/Manpage/btrfs(5)#MOUNT_OPTIONS)挂载相应的 Btrfs 分区。
