package glock

import (
	"testing"
	"time"
)

func memoryClients() (Client, Client) {
	return NewMemoryClient("client1"), NewMemoryClient("client2")
}

func TestAcquireMemory(t *testing.T) {
	c1, c2 := memoryClients()
	testAcquire(t, c1, c2, time.Millisecond)
}
