package glock

import (
	"testing"
	"time"
)

var lock = "lock"
var defData = "mydata"
var ttlLength = 30

type newClientFunc func(t *testing.T) Client

func options(scale time.Duration, ttl, maxWait int, data string) AcquireOptions {
	return AcquireOptions{
		TTL:     time.Duration(ttlLength) * scale,
		MaxWait: time.Duration(maxWait) * scale,
		Data:    data,
	}
}

func managers(c1, c2 Client, scale time.Duration) (*LockManager, *LockManager) {
	opts := options(scale, ttlLength, 0, defData)
	return NewLockManager(c1, opts), NewLockManager(c2, opts)
}

func info(t *testing.T, m *LockManager) *LockInfo {
	res, err := m.Info(lock)
	if err != nil {
		t.Fatalf("Error while getting lock info: '%s'", err)
	}
	return res
}

func testAcquire(t *testing.T, cfun newClientFunc, scale time.Duration) {
	m1, m2 := managers(cfun(t), cfun(t), scale)
	defer m1.ReleaseAll()
	defer m2.ReleaseAll()

	err := m1.Acquire(lock, AcquireOptions{})
	if err != nil {
		t.Fatalf("Cannot acquire lock: %s", err)
	}
	err = m2.Acquire(lock, AcquireOptions{})
	if err != ErrLockHeldByOtherClient {
		t.Fatalf("Wanted: '%s', got: '%s'", ErrLockHeldByOtherClient, err)
	}

	before := info(t, m1)
	// Acquire should refresh if lock already held and update Data
	err = m1.Acquire(lock, AcquireOptions{Data: "newdata"})
	if err != nil {
		t.Fatalf("Cannot acquire already acquired lock: %s", err)
	}
	after := info(t, m1)
	if after.TTL < before.TTL {
		t.Fatalf("Lock not refreshed? TTL %v < %v", after.TTL, before.TTL)
	}
}

func testClient(t *testing.T, cfun newClientFunc) {
	c1 := cfun(t)
	c2 := cfun(t)

	if c1.ID() == c2.ID() {
		t.Errorf("Both client have the same id: %s == %s", c1.ID(), c2.ID())
	}

	c1.SetID("myclient")
	id := c1.ID()
	if id != "myclient" {
		t.Errorf("SetID did not set ID correctly: 'myclient' expected, got %s", id)
	}

	// close should be idempotent, as well as Reconnect
	c1.Close()
	c1.Close()
	err := c1.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect error: %s", err)
	}
	err = c1.Reconnect()
	if err != nil {
		t.Fatalf("Reconnect error: %s", err)
	}

	c3 := c1.Clone()
	if &c1 == &c3 {
		t.Fatalf("Clone() should clone, not returning the same object! %p == %p", &c1, &c3)
	}
	if c1.ID() != c3.ID() {
		t.Errorf("Clone should have copied client ids '%s' != '%s'", c1.ID(), c3.ID())
	}
}
