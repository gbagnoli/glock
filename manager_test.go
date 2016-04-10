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

	if m1.Client().ID() == m2.Client().ID() {
		t.Fatalf("Both clients have the same id '%s'", m2.Client().ID())
	}

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
	if after.Data != "newdata" {
		t.Fatalf("Refreshing did not set new data: '%s' != 'newdata'", after.Data)
	}
	err = m1.SetData(lockName, "refresh")
	if err != nil {
		t.Fatalf("Got error while setting data: '%s'", err)
	}
	err = m1.Refresh(lockName)
	if err != nil {
		t.Fatalf("Error during refresh: '%s'", err)
	}
	st := info(t, m1)
	if st.Data != "refresh" {
		t.Fatalf("Data should have been set to 'refresh' after refresh, got '%s'", st.Data)
	}

	// Acquire an already acquired lock is equal to a refresh for the manager
	err = m1.SetData(lockName, "acquire-refresh")
	if err != nil {
		t.Fatalf("Got error while setting data: '%s'", err)
	}

	err = m1.SetData("nonexiting", "other")
	if err != ErrInvalidLock {
		t.Fatalf("Setting data on non existing lock should return '%s', got '%s'", ErrInvalidLock, err)
	}

	err = m1.Refresh("nonexisting")
	if err != ErrInvalidLock {
		t.Fatalf("Refreshing non-existing lock should return '%s', got '%s'", ErrInvalidLock, err)
	}

	_, err = m1.Info("nonexisting")
	if err != ErrInvalidLock {
		t.Fatalf("Info on non-existing lock should return '%s', got '%s'", ErrInvalidLock, err)
	}
}
