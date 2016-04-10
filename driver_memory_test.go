package glock

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
)

func memoryClient(t *testing.T) Client {
	return NewMemoryClient(gocql.TimeUUID().String())
}

func TestMemoryAcquire(t *testing.T) {
	testAcquire(t, memoryClient, time.Millisecond)
}

func TestMemoryClient(t *testing.T) {
	testClient(t, memoryClient)
}
