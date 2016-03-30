package glock

import (
	"sync"
	"time"
)

type locksDB struct {
	mtx   *sync.RWMutex
	locks map[string]*MemoryLock
}

var db *locksDB

func initDB() {
	if db == nil {
		db = &locksDB{
			mtx:   &sync.RWMutex{},
			locks: make(map[string]*MemoryLock),
		}
	}
}

type MemoryLock struct {
	name   string
	ttl    time.Duration
	data   string
	client *MemoryClient
	timer  *time.Timer
	expire time.Time
}

type MemoryClient struct {
	id string
}

func NewMemoryClient(id string) *MemoryClient {
	initDB()
	return &MemoryClient{id: id}
}

func (m *MemoryClient) Clone() Client {
	return NewMemoryClient(m.id)
}

func (m *MemoryClient) Close() {
	return
}

func (m *MemoryClient) Reconnect() error {
	return nil
}

func (m *MemoryClient) SetID(id string) {
	m.id = id
}

func (m *MemoryClient) ID() string {
	return m.id
}

func (m *MemoryClient) NewLock(name string) Lock {
	return &MemoryLock{name: name, client: m}
}

func (l *MemoryLock) Acquire(ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidTTL
	}
	l.ttl = ttl
	db.mtx.Lock()
	defer db.mtx.Unlock()

	_, ok := db.locks[l.name]
	switch ok {
	case true:
		return ErrLockHeldByOtherClient

	case false:
		l.timer = time.AfterFunc(l.ttl, func() {
			db.mtx.Lock()
			defer db.mtx.Unlock()
			delete(db.locks, l.name)
			l.timer = nil
		})
		l.expire = time.Now().Add(l.ttl)
		db.locks[l.name] = l
	}
	return nil
}

func (l *MemoryLock) Release() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	lock, ok := db.locks[l.name]
	if !ok {
		return ErrInvalidLock
	}
	if lock.client.id != l.client.id {
		return ErrLockHeldByOtherClient
	}
	lock.timer.Stop()
	delete(db.locks, l.name)
	l.timer = nil
	return nil
}

func (l *MemoryLock) Refresh() error {
	if l.ttl <= 0 {
		return ErrInvalidTTL
	}

	db.mtx.Lock()
	defer db.mtx.Unlock()
	lock, ok := db.locks[l.name]
	if !ok {
		return ErrInvalidLock
	}
	if lock.client.id != l.client.id {
		return ErrLockHeldByOtherClient
	}
	lock.timer.Reset(lock.ttl)
	lock.expire = time.Now().Add(lock.ttl)
	return nil
}

func (l *MemoryLock) RefreshTTL(ttl time.Duration) error {
	l.ttl = ttl
	return l.Refresh()
}

func (l *MemoryLock) Info() (*LockInfo, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	lock, ok := db.locks[l.name]
	if !ok {
		return &LockInfo{Name: l.name, Acquired: false}, nil
	}
	return &LockInfo{
		Name:     l.name,
		Acquired: true,
		Owner:    lock.client.id,
		TTL:      lock.expire.Sub(time.Now()),
		Data:     lock.data,
	}, nil
}

// SetData sets the data payload for the lock.
// The data is set into the backend only when the lock is acquired,
// so any call to this method after acquisition won't update the value.
func (l *MemoryLock) SetData(data string) {
	l.data = data
}
