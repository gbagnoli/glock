// +build cassandra

package glock

import (
	"flag"
	"testing"
	"time"
)

var host = flag.String("host", "localhost", "Cassandra host")
var keyspace = flag.String("keyspace", "glock_test", "Cassandra keyspace")
var username = flag.String("username", "", "Username to use when connecting")
var password = flag.String("password", "", "Password to use when connecting")

func cassandraClient(t *testing.T) Client {
	opts := CassandraOptions{
		Hosts:             []string{*host},
		KeySpace:          *keyspace,
		Username:          *username,
		Password:          *password,
		TableName:         "locks",
		ReplicationFactor: 1,
	}
	c, err := NewCassandraLockClient(opts)
	if err != nil {
		t.Fatalf("Cannot create cassandra client: %s", err)
	}
	return c
}

func cassandraClients(t *testing.T) (Client, Client) {
	return cassandraClient(t), cassandraClient(t)
}

func TestAcquireCassandra(t *testing.T) {
	c1, c2 := cassandraClients(t)
	testAcquire(t, c1, c2, time.Second)
}
