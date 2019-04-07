## 删除某worker的某镜像

先确定已经给tunasynctl写好config文件：`~/.config/tunasync/ctl.conf`

```
manager_addr = "127.0.0.1"
manager_port = 12345
ca_cert = ""
```

接着

```
$ tunasynctl disable -w <worker_id> <mirror_name>
$ tunasynctl flush
```


## 热重载 `worker.conf`

`$ tunasynctl reload -w <worker_id>`


e.g. 删除 `test_worker` 的 `elvish` 镜像：

1. 删除存放镜像的文件夹

2. 删除 `worker.conf` 中对应的 `mirror` 段落

3. 接着操作：

```
$ tunasynctl reload -w test_worker
$ tunasynctl disable -w test_worker elvish
$ tunasynctl flush
```

4. （可选）最后删除日志文件夹里的日志


## 删除worker

`$ tunasynctl rm-worker -w <worker_id>`

e.g. `$ tunasynctl rm-worker -w test_worker`


## 更新镜像的大小

`$ tunasynctl set-size -w <worker_id> <mirror_name> <size>`

其中，末尾的 <size> 参数，由操作者设定，或由某定时脚本生成

由于 `du -s` 比较耗时，故镜像大小可直接由rsync的日志文件读出
