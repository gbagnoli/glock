// +build memory

package glock

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
)

var memoryScale = time.Millisecond

func memoryClient(t *testing.T) Client {
	return NewMemoryClient(gocql.TimeUUID().String())
}

func TestMemoryManagerAcquire(t *testing.T) {
	testManagerAcquire(t, memoryClient, memoryScale)
}

func TestMemoryManagerAcquireWait(t *testing.T) {
	testManagerAcquireWait(t, memoryClient, memoryScale)
}

func TestMemoryManagerFailReleaseAll(t *testing.T) {
	testManagerFailReleaseAll(t, memoryClient, memoryScale)
}

func TestMemoryClient(t *testing.T) {
	testClient(t, memoryClient)
}

func TestMemoryLock(t *testing.T) {
	testLock(t, memoryClient, memoryScale)
}
