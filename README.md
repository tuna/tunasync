tunasync
========

[![Build Status](https://travis-ci.org/tuna/tunasync.svg?branch=dev)](https://travis-ci.org/tuna/tunasync)
[![Coverage Status](https://coveralls.io/repos/github/tuna/tunasync/badge.svg?branch=dev)](https://coveralls.io/github/tuna/tunasync?branch=dev)

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
	- [ ] use HTTP as communication protocol
	- [ ] implement manager as status server first, and use python worker
	- [ ] implement go worker
- Web frontend for `tunasync-manager`
	- [ ] start/stop/restart job
	- [ ] enable/disable mirror
	- [ ] view log
- [ ] config file structure
	- [ ] support multi-file configuration (`/etc/tunasync.d/mirror-enabled/*.conf`)

## Generate Self-Signed Certificate

Fisrt, create root CA

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
