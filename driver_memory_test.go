package glock

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
)

func memoryClient(t *testing.T) Client {
	return NewMemoryClient(gocql.TimeUUID().String())
}

func TestMemoryManagerAcquire(t *testing.T) {
	testManagerAcquire(t, memoryClient, time.Millisecond)
}

func TestMemoryClient(t *testing.T) {
	testClient(t, memoryClient)
}

func TestMemoryLock(t *testing.T) {
	testLock(t, memoryClient, time.Millisecond)
}
