glock
=====

[![Build Status](https://travis-ci.org/gbagnoli/glock.png?branch=master)](https://travis-ci.org/gbagnoli/glock)
[![GoDoc](http://godoc.org/github.com/gbagnoli/glock?status.png)](http://godoc.org/github.com/gbagnoli/glock)

Package glock implements locking against a variety of backends for the Go
programming language

**WARN**: this package is under heavy develpment, API is not stable yet.

Backends
--------

* Redis (single server)

  Simple [Redis](http://redis.io/) implementation. Requires redis >= 2.6 as it
  uses [lua scripting](http://redis.io/commands/eval).  
  This implementation is safe only if used againt a single master, with no
  replication.

* Cassandra

  [Cassandra](http://cassandra.apache.org/) implementation, inspired by
  datastax's "[Consensus on Cassandra](http://cassandra.apache.org/)" blogpost.  
  Requires cassandra >= 2.0 as it uses lightweight transactions.

* Memory
  Naive in-process implementation, only useful for testing.

Installation
------------

```
go get github.com/gbagnoli/glock
```

Tests
-----

By default, tests runs only using the `memory` driver.
To run test for one or more specific backend, use build tags.

```
go test -tags="redis cassandra"
```

Roadmap
-------

1. Add more tests
1. Add more documentation
1. Add more backends (in no particular order)
  * [ZooKeeper](https://zookeeper.apache.org/)
  * [etcd](https://github.com/coreos/etcd)
  * [consul](https://www.consul.io/)
  * [redis redlock](http://redis.io/topics/distlock)
1. Stabilize interface.

Example
-------

See [glock-example](./glock-example)
