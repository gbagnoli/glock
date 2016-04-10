package glock

import (
	"testing"
	"time"
)

var lockName = "lock"
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
	res, err := m.Info(lockName)
	if err != nil {
		t.Fatalf("Error while getting lock info: '%s'", err)
	}
	return res
}

func testManagerAcquire(t *testing.T, cfun newClientFunc, scale time.Duration) {
	m1, m2 := managers(cfun(t), cfun(t), scale)
	defer m1.ReleaseAll()
	defer m2.ReleaseAll()

	err := m1.Acquire(lockName, AcquireOptions{})
	if err != nil {
		t.Fatalf("Cannot acquire lock: %s", err)
	}
	err = m2.Acquire(lockName, AcquireOptions{})
	if err != ErrLockHeldByOtherClient {
		t.Fatalf("Wanted: '%s', got: '%s'", ErrLockHeldByOtherClient, err)
	}

	before := info(t, m1)
	// Acquire should refresh if lock already held and update Data
	err = m1.Acquire(lockName, AcquireOptions{Data: "newdata"})
	if err != nil {
		t.Fatalf("Cannot acquire already acquired lock: %s", err)
	}
	after := info(t, m1)
	if after.TTL < before.TTL {
		t.Fatalf("Lock not refreshed? TTL %v < %v", after.TTL, before.TTL)
	}
}
