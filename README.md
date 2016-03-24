tunasync
========

## Design

```
# Architecture

- Manager: Centural instance on status and job management
- Worker: Runs mirror jobs


+----------+  +---+   worker configs   +---+    +----------+     +----------+
|  Status  |  |   |+-----------------> | w +--->|  mirror  +---->|  mirror  |
|  Manager |  |   |                    | o |    |  config  |     | provider |
+----------+  | W |  start/stop job    | r |    +----------+     +----+-----+
              | E |+-----------------> | k |                          |
+----------+  | B |                    | e |       +------------+     |
|   Job    |  |   |   update status    | r |<------+ mirror job |<----+
|Controller|  |   | <-----------------+|   |       +------------+
+----------+  +---+                    +---+


# Job Run Process

+-----------+     +-----------+    +-------------+     +--------------+
|  pre-job  +--+->|  job run  +--->|   post-job  +-+-->| post-success |
+-----------+  ^  +-----------+    +-------------+ |   +--------------+
			   |                                   |
			   |      +-----------------+          |
			   +------+    post-fail    |<---------+
					  +-----------------+
```

## TODO

- [ ] split to `tunasync-manager` and `tunasync-worker` instances
	- use HTTP as communication protocol
- Web frontend for `tunasync-manager`
	- [ ] start/stop/restart job
	- [ ] enable/disable mirror
	- [ ] view log
- [ ] config file structure
	- [ ] support multi-file configuration (`/etc/tunasync.d/mirror-enabled/*.conf`)

