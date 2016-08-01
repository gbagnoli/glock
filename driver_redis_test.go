// +build redis

package glock

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stvp/tempredis"
)

const port = "12345"

var namespace = flag.String("namspace", "glock:tests", "Redis keys namespace")
var server *tempredis.Server

func TestMain(m *testing.M) {
	var err error
	config := map[string]string{
		"port": port,
		"bind": "127.0.0.1",
	}
	server, err = tempredis.Start(config)
	if err != nil {
		panic(err)
	}
	result := m.Run()
	server.Term()
	os.Exit(result)
}

func redisClient(t *testing.T) Client {
	opts := RedisOptions{
		Network:   "tcp",
		Address:   "127.0.0.1" + ":" + port,
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
