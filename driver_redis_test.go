// +build redis

package glock

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stvp/tempredis"
)

var namespace = flag.String("namspace", "glock:tests", "Redis keys namespace")
var server *tempredis.Server

func TestMain(m *testing.M) {
	var err error
	server, err = tempredis.Start(nil)
	if err != nil {
		panic(err)
	}
	result := m.Run()
	server.Term()
	os.Exit(result)
}

func redisClient(t *testing.T) Client {
	opts := RedisOptions{
		Network:   "unix",
		Address:   server.Socket(),
		Namespace: *namespace,
	}
	c1, err := NewRedisClient(opts)
	if err != nil {
		t.Fatalf("Cannot create redis client: %s", err)
	}
	return c1
}

func TestRedisClient(t *testing.T) {
	testClient(t, redisClient)
}

func TestRedisLock(t *testing.T) {
	testLock(t, redisClient, time.Millisecond)
}
