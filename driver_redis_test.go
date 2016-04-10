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

func redisClients(t *testing.T) (Client, Client) {
	return redisClient(t), redisClient(t)
}

func TestAcquireRedis(t *testing.T) {
	c1, c2 := redisClients(t)
	testAcquire(t, c1, c2, time.Millisecond)
}
