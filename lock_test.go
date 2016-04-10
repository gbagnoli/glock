package glock

import (
	"testing"
	"time"
)

func testLock(t *testing.T, cfun newClientFunc, scale time.Duration) {
	c1 := cfun(t)
	c2 := cfun(t)

	lock1 := c1.NewLock(lockName)
	lock2 := c2.NewLock(lockName)

	lock1.SetData("client1")
	lock2.SetData("client2")

	err := lock1.Acquire(time.Duration(ttlLength) * scale)
	if err != nil {
		t.Fatalf("Cannot acquire lock '%s': %s", lockName, err)
	}

	// Re-acquire the same lock twice is an error
	err = lock1.Acquire(time.Duration(ttlLength) * scale)
	if err != ErrLockHeldByOtherClient {
		t.Errorf("Expected error '%s', got %s", ErrLockHeldByOtherClient, err)
	}

	// Other client should see the lock as acquired
	err = lock2.Acquire(time.Duration(ttlLength) * scale)
	if err != ErrLockHeldByOtherClient {
		t.Errorf("Expected error '%s', got %s", ErrLockHeldByOtherClient, err)
	}

	// client 2 cannot release the lock as it's held by client 1
	err = lock2.Release()
	if err != ErrLockNotOwned {
		t.Fatalf("Releasing a lock held by another client should return '%s', got: '%s'", ErrLockNotOwned, err)
	}

	// Refreshing a lock held by another client
	err = lock2.Refresh()
	if err != ErrLockNotOwned {
		t.Fatalf("Refreshing a lock not held should return '%s', got: '%s'", ErrLockNotOwned, err)
	}

	// Refreshing a lock acquired should succeed
	err = lock1.Refresh()
	if err != nil {
		t.Fatalf("Error while refreshing lock: '%s'", err)
	}

	// Refreshing a lock should check for the TTL
	err = lock1.RefreshTTL(scale / 2)
	if err != ErrInvalidTTL {
		t.Fatalf("Expected error '%s' with TTL %v, got '%s'", ErrInvalidTTL, scale/2, err)
	}

	info1, err := lock1.Info()
	if err != nil {
		t.Fatalf("Error while getting lock info: %s", err)
	}

	info2, err := lock2.Info()
	if err != nil {
		t.Fatalf("Error while getting lock info: %s", err)
	}

	if info1.Name != lockName {
		t.Errorf("LockInfo Name should be equal to the lock name '%s', got '%s'", lockName, info1.Name)
	}
	if info1.Name != info2.Name {
		t.Errorf("Info should return the same name for both clients: '%s' != '%s'",
			info1.Name, info2.Name)
	}
	if info1.TTL > time.Duration(ttlLength)*scale || info1.TTL <= 0 {
		t.Errorf("Invalid TTL returned by lock.Info() : %v", info1.TTL)
	}
	if info1.Acquired != true {
		t.Errorf("Lock is held by client1, info.Acquired should be true, got %v", info1.Acquired)
	}
	if info2.Acquired != true {
		t.Errorf("Lock is held by client1, info.Acquired should be true, got %v", info2.Acquired)
	}

	if info1.Data != "client1" {
		t.Errorf("Expected data 'client1', got '%s'", info1.Data)
	}

	if info2.Data != info1.Data {
		t.Errorf("lock.Info() should return the data set by the client who acquired the lock ('client1'), got '%s'", info2.Data)
	}

	if info1.Owner != c1.ID() {
		t.Errorf("info.Owner should be equal to the client ID of the client owning the lock ('%s'), got '%s'", c1.ID(), info1.Owner)
	}
	if info2.Owner != c1.ID() {
		t.Errorf("info.Owner should be equal to the client ID of the client owning the lock ('%s'), got '%s'", c1.ID(), info2.Owner)
	}

	// refreshing should update TTL and Data
	lock1.SetData("newdata")
	err = lock1.RefreshTTL(info1.TTL * 2)
	if err != nil {
		t.Fatalf("Error in RefreshTTL: '%s'", err)
	}
	info3, err := lock1.Info()
	if err != nil {
		t.Fatalf("Error in Info: '%s'", err)
	}
	if info3.TTL <= info1.TTL {
		t.Errorf("Lock not refreshed? %v <= %v should be closer to %v",
			info3.TTL, info1.TTL, info1.TTL*2)
	}
	if info3.Data != "newdata" {
		t.Errorf("Refresh did not refresh data, expected 'newdata' got '%s'", info3.Data)
	}

	err = lock1.Release()
	if err != nil {
		t.Fatalf("Cannot release lock '%s': %s", lockName, err)
	}
	// Acquiring the lock with an invalid TTL should result in an error
	err = lock1.Acquire(scale / 2)
	if err != ErrInvalidTTL {
		t.Fatalf("Expected error '%s' with TTL %v, got '%s'", ErrInvalidTTL, scale/2, err)
	}

	// Releasing a lock twice is an error
	err = lock1.Release()
	if err != ErrLockNotOwned {
		t.Fatalf("Releasing a lock twice should return '%s', got: '%s'", ErrLockNotOwned, err)
	}

	// Refreshing a lock not held is an error
	err = lock1.Refresh()
	if err != ErrLockNotOwned {
		t.Fatalf("Refreshing a lock not held should return '%s', got: '%s'", ErrLockNotOwned, err)
	}

	// Getting info on a non-existing/already release lock should not return an error
	info1, err = lock1.Info()
	if err != nil {
		t.Fatalf("Error in Info: '%s'", err)
	}
	if info1.Acquired == true {
		t.Error("Lock is not acquired but info.Acquired = true")
	}

	// Finally, locks should expire.
	lock1.Acquire(scale)
	time.Sleep(scale * 2)
	info1, err = lock1.Info()
	if err != nil {
		t.Fatalf("Error in Info: '%s'", err)
	}
	if info1.Acquired != false {
		t.Fatalf("Lock should be expired but info.Acquired = true")
	}
	err = lock1.Refresh()
	if err != ErrLockNotOwned {
		t.Fatalf("Lock should be expired but refresh was succesful: %s", err)
	}
	err = lock1.Release()
	if err != ErrLockNotOwned {
		t.Fatalf("Lock should be expired but release was succesful: %s", err)
	}
}
