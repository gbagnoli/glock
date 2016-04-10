// +build redis

package glock

import (
	"flag"
	"testing"
	"time"
)

var network = flag.String("network", "tcp", "Redis network")
var address = flag.String("address", "localhost:6379", "Redis addr")
var namespace = flag.String("namspace", "glock:tests", "Redis keys namespace")

func redisClient(t *testing.T) Client {
	opts := RedisOptions{
		Network:   *network,
		Address:   *address,
		Namespace: *namespace,
	}
	c1, err := NewRedisClient(opts)
	if err != nil {
		t.Fatalf("Cannot create redis client: %s", err)
	}
	return c1
}

func TestRedisManagerAcquire(t *testing.T) {
	testManagerAcquire(t, redisClient, time.Millisecond)
}

func TestRedisClient(t *testing.T) {
	testClient(t, redisClient)
}

func TestRedisLock(t *testing.T) {
	testLock(t, redisClient, time.Millisecond)
}
