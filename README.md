tunasync
========

[![Build Status](https://travis-ci.org/tuna/tunasync.svg?branch=dev)](https://travis-ci.org/tuna/tunasync)
[![Coverage Status](https://coveralls.io/repos/github/tuna/tunasync/badge.svg?branch=dev)](https://coveralls.io/github/tuna/tunasync?branch=dev)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
![GPLv3](https://img.shields.io/badge/license-GPLv3-blue.svg)

## Get Started

- [中文文档](https://github.com/tuna/tunasync/blob/master/docs/zh_CN/get_started.md)

## Download

Pre-built binary for Linux x86_64 is available at [Github releases](https://github.com/tuna/tunasync/releases/latest).

## Design

```
# Architecture

- Manager: Central instance for status and job management
- Worker: Runs mirror jobs

+------------+ +---+                  +---+
| Client API | |   |    Job Status    |   |    +----------+     +----------+ 
+------------+ |   +----------------->|   |--->|  mirror  +---->|  mirror  | 
+------------+ |   |                  | w |    |  config  |     | provider | 
| Worker API | | H |                  | o |    +----------+     +----+-----+ 
+------------+ | T |   Job Control    | r |                          |       
+------------+ | T +----------------->| k |       +------------+     |       
| Job/Status | | P |   Start/Stop/... | e |       | mirror job |<----+       
| Management | | S |                  | r |       +------^-----+             
+------------+ |   |   Update Status  |   |    +---------+---------+         
+------------+ |   <------------------+   |    |     Scheduler     |
|   BoltDB   | |   |                  |   |    +-------------------+
+------------+ +---+                  +---+


# Job Run Process


PreSyncing           Syncing                               Success
+-----------+     +-----------+    +-------------+     +--------------+
|  pre-job  +--+->|  job run  +--->|  post-exec  +-+-->| post-success |
+-----------+  ^  +-----------+    +-------------+ |   +--------------+
			   |                                   |
			   |      +-----------------+          | Failed
			   +------+    post-fail    |<---------+
			          +-----------------+
```


## Building

Setup GOPATH like [this](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable).

Then:

```
go get -d github.com/tuna/tunasync/cmd/tunasync
cd $GOPATH/src/github.com/tuna/tunasync
make
```

If you have multiple `GOPATH`s, replace the `$GOPATH` with your first one.
