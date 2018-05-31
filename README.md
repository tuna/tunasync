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

## Generate Self-Signed Certificate

First, create root CA

```
openssl genrsa -out rootCA.key 2048
openssl req -x509 -new -nodes -key rootCA.key -days 365 -out rootCA.crt
```

Create host key

```
openssl genrsa -out host.key 2048
```

Now create CSR, before that, write a `req.cnf`

```
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
countryName = Country Name (2 letter code)
countryName_default = CN
stateOrProvinceName = State or Province Name (full name)
stateOrProvinceName_default = BJ
localityName = Locality Name (eg, city)
localityName_default = Beijing
organizationalUnitName  = Organizational Unit Name (eg, section)
organizationalUnitName_default  = TUNA
commonName = Common Name (server FQDN or domain name)
commonName_default = <server_FQDN>
commonName_max  = 64

[v3_req]
# Extensions to add to a certificate request
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = <server_FQDN_1>
DNS.2 = <server_FQDN_2>
```

Substitute `<server_FQDN>` with your server's FQDN, then run

```
openssl req -new -key host.key -out host.csr -config req.cnf
```

Finally generate and sign host cert with root CA

```
openssl x509 -req -in host.csr -CA rootCA.crt -CAkey rootCA.key -CAcreateserial -out host.crt -days 365 -extensions v3_req -extfile req.cnf
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
