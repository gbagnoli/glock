package glock

import "time"

// LockManager manages all the locks for a single client
type LockManager struct {
	defaultTTL time.Duration
	client     Client
	locks      map[string]Lock
	hb         map[string]chan bool
}

// NewLockManager returns a new LockManager for the given client.
func NewLockManager(client Client, defaultTTL time.Duration) *LockManager {
	return &LockManager{
		defaultTTL,
		client,
		make(map[string]Lock),
		make(map[string]chan bool),
	}
}

// TryAcquire tries to acquire the lock with the given name using the default TTL for the manager
// it will return immediately if the lock cannot be acquired
func (m *LockManager) TryAcquire(lockName string) error {
	return m.TryAcquireTTL(lockName, m.defaultTTL)
}

// Info returns information about a lock with the given name
func (m *LockManager) Info(lockName string) (*LockInfo, error) {
	lock, ok := m.locks[lockName]
	if !ok {
		return nil, ErrInvalidLock
	}
	info, err := lock.Info()
	if err != nil {
		return nil, err
	}
	return info, nil
}

// TryAcquireTTL is like TryAcquire, but with a custom TTL for this lock.
func (m *LockManager) TryAcquireTTL(lockName string, ttl time.Duration) error {
	lock := m.client.NewLock(lockName)
	err := lock.Acquire(ttl)
	if err != nil {
		return err
	}
	m.locks[lockName] = lock
	return nil
}

// Acquire tries to acquire the lock with the given name using the default TTL for the manager.
// If the lock cannot be acquired, it will wait up to maxWait for the lock to be released by the owner.
func (m *LockManager) Acquire(lockName string, maxWait time.Duration) error {
	return m.AcquireTTL(lockName, m.defaultTTL, maxWait)
}

// AcquireTTL works exactly like Acquire, but with a custom TTL for this lock.
func (m *LockManager) AcquireTTL(lockName string, ttl time.Duration, maxWait time.Duration) error {
	var waited time.Duration
	lock := m.client.NewLock(lockName)
	for {
		err := m.TryAcquireTTL(lockName, ttl)
		if err == nil {
			return nil
		}
		if err != ErrLockHeldByOtherClient {
			return err
		}
		info, err := lock.Info()
		if err != nil {
			return err
		}
		waited = waited + info.TTL
		if waited > maxWait {
			return ErrLockHeldByOtherClient
		}
		time.Sleep(info.TTL)
	}
}

// Release releases a lock with the given name. The lock must be held by the current manager.
// Any eventual heartbeating will be stopped as well.
func (m *LockManager) Release(lockName string) error {
	err := m.locks[lockName].Release()
	m.StopHeartbeat(lockName)
	delete(m.locks, lockName)
	return err
}

// ReleaseAll releases all the locks held by the manager.
func (m *LockManager) ReleaseAll() map[string]error {
	results := make(map[string]error)
	var err error
	for n := range m.locks {
		err = m.Release(n)
		if err != nil {
			results[n] = err
		}
	}
	return results
}

func heartbeat(client Client, lockName string, ttl time.Duration, control chan bool) {
	client.Reconnect()
	defer client.Close()
	freq := time.Duration(ttl / 2)
	enlapsed := time.Duration(0)
	sleeptime := 10 * time.Millisecond
	if sleeptime > freq {
		sleeptime = freq
	}

	lock := client.NewLock(lockName)
	for {
		select {
		case <-control:
			return

		default:
			if enlapsed >= freq {
				start := time.Now()
				err := lock.RefreshTTL(ttl)
				if err != nil {
					panic(err)
				}
				s := sleeptime - (time.Now().Sub(start))
				time.Sleep(s)
				enlapsed = s
			} else {
				time.Sleep(sleeptime)
				enlapsed += sleeptime
			}
		}
	}
}

// StartHeartbeat starts a goroutine in the backgroud that will refresh the lock
// The lock will be refreshed every ttl/2 or whichever is greater.
// The background goroutine will panic() if the lock cannot be refreshed for any reason
// The background goroutine will run forever until StopHeartbeat is called or the lock released.
func (m *LockManager) StartHeartbeat(lockName string) error {
	info, err := m.Info(lockName)
	if err != nil {
		return err
	}
	m.hb[lockName] = make(chan bool)
	go heartbeat(m.client, lockName, info.TTL, m.hb[lockName])
	return nil
}

// StopHeartbeat will stop the background gororoutine, if any, that is heartbeating the given lock
func (m *LockManager) StopHeartbeat(lockName string) {
	if c, ok := m.hb[lockName]; ok {
		c <- true
		delete(m.hb, lockName)
	}
}
