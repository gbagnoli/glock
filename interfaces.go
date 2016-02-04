package glock

import (
	"errors"
	"time"
)

// Client represents a client that can own locks in the store
type Client interface {

	// ID returns the client id
	ID() string

	// SetID set the client id
	SetID(id string)

	// Reconnect reconnects to the store, or connects if not connected
	Reconnect() error

	// Close closes the connection to the store
	Close()

	// NewLock returns a new lock object
	NewLock(name string) Lock
}

// Lock represent a lock in the store
type Lock interface {

	// Info returns a LockInfo struct with information about this lock
	Info() (*LockInfo, error)

	// Acquire tries to acquire the lock for a specified duration
	// The lock must not be locked.
	Acquire(ttl time.Duration) error

	// Refresh extends the validity of the lock by its ttl
	// The lock must be acquired by the current client.
	Refresh() error

	// RefreshTTL extends the validity of the lock by the given ttl.
	// The lock must be acquired by the current client
	RefreshTTL(ttl time.Duration) error

	// Release removed the lock from the store.
	// The lock must be owned by the current client
	Release() error
}

// LockInfo represent information about a given lock
type LockInfo struct {
	// Name is the lock name
	Name string
	// Acquired is true if the lock is acquired, false otherwise
	Acquired bool
	// Owner if the ClientID of the client owning the lock, if any
	Owner string
	// The remaining TTL until the lock is automatically expired
	TTL time.Duration
}

var (
	// ErrInvalidTTL is returnend when the TTL specified is not a valid TTL
	ErrInvalidTTL = errors.New("Invalid ttl value")
	// ErrLockHeldByOtherClient is returned when the operation cannot be performed as the lock is
	// not held by the current client
	ErrLockHeldByOtherClient = errors.New("Lock held by other client")
	// ErrInvalidLock is returned when the lock name is invalid
	ErrInvalidLock = errors.New("Invalid lock name")
	// ErrLockNotOwned is returned when either the lock is not existing or held by another client
	ErrLockNotOwned = errors.New("Invalid lock name")
)
