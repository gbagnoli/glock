package glock

import (
	"errors"
	"io/ioutil"
	"log"
	"time"
)

// LockManager manages all the locks for a single client
type LockManager struct {
	Logger *log.Logger
	opts   AcquireOptions
	client Client
	locks  map[string]Lock
	hb     map[string]chan error
}

// AcquireOptions allows to set options during lock acquisition.
type AcquireOptions struct {
	// TTL is the ttl of the lock. If <= 0 the manager defaultTTL is used
	TTL time.Duration
	// How long to wait for the lock to be acquired.
	MaxWait time.Duration
	// The Data to set with the lock.
	Data string
}

// NewLockManager returns a new LockManager for the given client.
// By default, logging is sent to /dev/null. You must call SetOutput() on
// the Logger instance if you want logging to be sent somewhere.
func NewLockManager(client Client, opts AcquireOptions) *LockManager {
	return &LockManager{
		log.New(ioutil.Discard, "glock: ", log.LstdFlags|log.LUTC),
		opts,
		client,
		make(map[string]Lock),
		make(map[string]chan error),
	}
}

// Client returns the current lock client in use
func (m *LockManager) Client() Client {
	return m.client
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

func (m *LockManager) acquire(lockName string, opts AcquireOptions) error {
	lock := m.client.NewLock(lockName)
	lock.SetData(opts.Data)
	err := lock.Acquire(opts.TTL)
	if err != nil {
		m.Logger.Printf("client %s: Cannot acquire lock '%s': %s",
			m.client.ID(), lockName, err.Error())
		return err
	}
	m.Logger.Printf("client %s: Acquired lock '%s' for %v", m.client.ID(), lockName, opts.TTL)
	m.locks[lockName] = lock
	return nil
}

// Acquire tries to acquire the lock with the given name using the default TTL
// for the manager. If the lock cannot be acquired, it will wait up to MaxWait
// for the lock to be released by the owner.
// If this manager instance already has acquired this lock, this action is a no-op.
func (m *LockManager) Acquire(lockName string, opts AcquireOptions) error {

	if _, ok := m.locks[lockName]; ok {
		return nil
	}

	var waited time.Duration
	lock := m.client.NewLock(lockName)

	if opts.TTL <= 0 {
		opts.TTL = m.opts.TTL
	}

	if opts.Data == "" {
		opts.Data = m.opts.Data
	}

	if opts.MaxWait <= 0 {
		opts.MaxWait = m.opts.MaxWait
	}

	for {
		init := time.Now()
		err := m.acquire(lockName, opts)
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

		if waited >= opts.MaxWait {
			m.Logger.Printf("client %s: Cannot acquire lock '%s' after %v",
				m.client.ID(), lockName, waited)
			return ErrLockHeldByOtherClient
		}

		wait := info.TTL - time.Since(init)
		if waited+wait > opts.MaxWait {
			wait = opts.MaxWait - waited
		}

		time.Sleep(wait)
		waited = waited + wait
	}
}

// Release releases a lock with the given name. The lock must be held by the current manager.
// Any eventual heartbeating will be stopped as well.
func (m *LockManager) Release(lockName string) error {
	var err error
	m.Logger.Printf("client %s: Releasing lock '%s'", m.client.ID(), lockName)
	if lock, ok := m.locks[lockName]; ok {
		m.StopHeartbeat(lockName)
		err = lock.Release()
		delete(m.locks, lockName)
	}
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

func heartbeat(client Client, logger *log.Logger, lockName string, ttl time.Duration, control chan error) {
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
					logger.Printf("client %s: heartbeat -- FATAL cannot refresh lock '%s': %s",
						client.ID(), lockName, err.Error())
					select {
					case control <- err:
						return
					case <-time.After(sleeptime):
						panic(err)
					}
				}
				logger.Printf("client %s: heartbeat -- refreshed lock '%s' for %v",
					client.ID(), lockName, ttl)
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

// StartHeartbeat starts a goroutine in the backgroud that will refresh the
// lock The lock will be refreshed every ttl/2 or whichever is greater.  The
// background goroutine will panic() if the lock cannot be refreshed for any
// reason The background goroutine will run forever until StopHeartbeat is
// called or the lock released.  It will return a channel to signal if the lock
// cannot be refreshed during heartbeats (before panicking)
func (m *LockManager) StartHeartbeat(lockName string) (<-chan error, error) {
	info, err := m.Info(lockName)
	if err != nil {
		return nil, err
	}
	m.Logger.Printf("client %s: Starting heartbeats for lock '%s' every %v", m.client.ID(),
		lockName, info.TTL/2)
	m.hb[lockName] = make(chan error)
	go heartbeat(m.client.Clone(), m.Logger, lockName, info.TTL, m.hb[lockName])
	return m.hb[lockName], nil
}

// StopHeartbeat will stop the background gororoutine, if any, that is heartbeating the given lock
func (m *LockManager) StopHeartbeat(lockName string) {
	if c, ok := m.hb[lockName]; ok {
		m.Logger.Printf("client %s: Stopping heartbeats for lock '%s'", m.client.ID(),
			lockName)
		c <- errors.New("")
		close(c)
		delete(m.hb, lockName)
	}
}
