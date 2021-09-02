# About Tunasync and cgroup

Optionally, tunasync can be integrated with cgroup to have better control and tracking processes started by mirror jobs. Also, limiting memory usage of a mirror job also requires cgroup support. 

## How cgroup are utilized in tunasync?

If cgroup are enabled globally, all the mirror jobs, except those running in docker containers, are run in separate cgroups. If `mem_limit` is specified, it will be applied to the cgroup. For jobs running in docker containers, `mem_limit` is applied via `docker run` command.


## Tl;dr: What's the recommended configuration?

### If you are using v1 (legacy, hybrid) cgroup hierarchy:

`tunasync-worker.service`:

```
[Unit]
Description = TUNA mirrors sync worker
After=network.target

[Service]
Type=simple
User=tunasync
PermissionsStartOnly=true
ExecStartPre=/usr/bin/cgcreate -t tunasync -a tunasync -g memory:tunasync
ExecStart=/home/bin/tunasync worker -c /etc/tunasync/worker.conf --with-systemd
ExecReload=/bin/kill -SIGHUP $MAINPID
ExecStopPost=/usr/bin/cgdelete memory:tunasync

[Install]
WantedBy=multi-user.target
```

`worker.conf`:

``` toml
[cgroup]
enable = true
group = "tunasync"
```

### If you are using v2 (unified) cgroup hierarchy:

`tunasync-worker.service`:

```
[Unit]
Description = TUNA mirrors sync worker
After=network.target

[Service]
Type=simple
User=tunasync
ExecStart=/home/bin/tunasync worker -c /etc/tunasync/worker.conf --with-systemd
ExecReload=/bin/kill -SIGHUP $MAINPID
Delegate=yes

[Install]
WantedBy=multi-user.target
```

`worker.conf`:

``` toml
[cgroup]
enable = true
```


## Two versions of cgroups

Due to various of reasons, there are two versions of cgroups in the kernel, which are incompatible with each other. Most of the current linux distributions adopts systemd as the init system, which relies on cgroup and is responsible for initializing cgroup. As a result, the selection of the version of cgroups is mainly decided by systemd. Since version 243, the "unified" cgroup hierarchy setup has become the default. 

Tunasync can automatically detect which version of cgroup is in use and enable the corresponding operating interface, but due to the fact that systemd behaves slightly differently  in the two cases, different configurations for tunasync are recomended.

## Two modes of group name discovery

Two modes of group name discovery are provided: implicit mode and manual mode.

### Manual Mode

In this mode, the administrator should 1. manually create an empty cgroup (for cgroup v2 unified hierarchy) or empty cgroups in certain controller subsystems with the same name (for cgroup v1 hybird hierarchy); 2. change the ownership of the cgroups to the running user of the tunasync worker; and 3. specify the path in the configuration. On start, tunasync will automatically detect which controllers are enabled (for v1) or enable needed controllers (for v2).

Example 1:

``` bash
# suppose we have cgroup v1
sudo mkdir -p /sys/fs/cgroup/cpu/test/tunasync
sudo mkdir -p /sys/fs/cgroup/memory/test/tunasync
sudo chown -R tunasync:tunasync /sys/fs/cgroup/cpu/test/tunasync
sudo chown -R tunasync:tunasync /sys/fs/cgroup/memory/test/tunasync

# in worker.conf, we have group = "/test/tunasync" or "test/tunasync"
tunasync worker -c /path/to/worker.conf
```

In the above scenario, tunasync will detect the enabled subsystem controllers are cpu and memory. When running a mirror job named `foo`, sub-cgroups will be created in both `/sys/fs/cgroup/cpu/test/tunasync/foo` and `/sys/fs/cgroup/memory/test/tunasync/foo`.

Example 2 (not recommended):

``` bash
# suppose we have cgroup v2
sudo mkdir -p /sys/fs/cgroup/test/tunasync
sudo chown -R tunasync:tunasync /sys/fs/cgroup/test/tunasync

# in worker.conf, we have group = "/test/tunasync" or "test/tunasync"
tunasync worker -c /path/to/worker.conf
```

In the above scenario, tunasync will directly use the cgroup `/sys/fs/cgroup/test/tunasync`. In most cases, due to the design of cgroupv2, since tunasync is not running as root, tunasync won't have the permission to move the processes it starts to the correct cgroup. That's because cgroup2 requires the operating process should also have the write permission of the common ancestor of the source group and the target group when moving processes between groups. So this example is only for demonstration of the functionality and you should prevent it.

### Implicit mode

In this mode, tunasync will use the cgroup it is currently running in and create sub-groups for jobs in that group. Tunasync will first create a sub-group named `__worker` in that group, and move itself in the `__worker` sub-group, to prevent processes in non-leaf cgroups.

Mostly, this mode is cooperated with the `Delegate=yes` option of the systemd service configuration of tunasync, which will permit the running process to self-manage the cgroup the service in running in. Due to security considerations, systemd won't give write permissions of the current running cgroups to the service when using v1 (legacy, hybrid) cgroup hierarchy and non-root user, so it is more meaningful to use this mode with v2 cgroup hierarchy.


## Configruation 

``` toml
[cgroup]
enable = true
base_path = "/sys/fs/cgroup"
group = "tunasync"
subsystem = "memory"
```

The defination of the above options is:

* `enable`: `Bool`, specifies whether cgroup is enabled. When cgroup is disabled, `memory_limit` for non-docker jobs will be ignored, and the following options are also ignored.
* `group`: `String`, specifies the cgroup tunasync will use. When not provided, or provided with empty string, cgroup discovery will work in "Implicit mode", i.e. will create sub-cgroups in the current running cgroup. Otherwise, cgroup discovery will work in "Manual mode", where tunasync will create sub-cgroups in the specified cgroup.
* `base_path`: `String`, ignored. It originally specifies the mounting path of cgroup filesystem, but for making everything work, it is now required that the cgroup filesystem should be mounted at its default path(`/sys/fs/cgroup`).
* `subsystem `: `String`, ignored. It originally specifies which cgroupv1 controller is enabled and now becomes meaningless since the discovery is now automatic.

## References:

* [https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html]()
* [https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/index.html]()
* [https://systemd.io/CGROUP_DELEGATION/]()
* [https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html#Delegate=]()
