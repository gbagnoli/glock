package glock

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

const (
	createKs    = `CREATE KEYSPACE IF NOT EXISTS %s WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : %d } AND DURABLE_WRITES=true`
	createTable = `CREATE TABLE IF NOT EXISTS %s.%s (name text PRIMARY KEY, owner text, data text)`
	acquireQ    = `INSERT INTO %s.%s (name, owner, data) VALUES (?, ?, ?) IF NOT EXISTS USING TTL %d`
	releaseQ    = `DELETE FROM %s.%s WHERE name = ? IF owner = ?`
	refreshQ    = `UPDATE %s.%s USING TTL %d set owner = ?, data = ? WHERE name = ? IF owner = ?`
	infoQ       = `SELECT owner, TTL(owner), data FROM %s.%s WHERE name = ?`
)

// CassandraOptions represents options for connecting to cassandra
type CassandraOptions struct {
	Hosts             []string
	KeySpace          string
	Username          string
	Password          string
	TableName         string
	ReplicationFactor int
}

// CassandraClient is the Client implementation for cassandra
type CassandraClient struct {
	cluster     *gocql.ClusterConfig
	hosts       []string
	keyspace    string
	table       string
	clientID    string
	session     *gocql.Session
	consistency gocql.Consistency
}

// CassandraLock is the Lock implementation for cassandra
type CassandraLock struct {
	name   string
	owner  string
	ttl    time.Duration
	client *CassandraClient
	data   string
}

// NewCassandraLockClient creates a new client from options
func NewCassandraLockClient(opts CassandraOptions) (*CassandraClient, error) {
	if opts.ReplicationFactor <= 0 {
		opts.ReplicationFactor = 1
	}
	consistency := gocql.Quorum

	if opts.ReplicationFactor == 0 {
		consistency = gocql.One
	}
	c := CassandraClient{nil, opts.Hosts, "", "", "", nil, consistency}

	c.cluster = gocql.NewCluster(opts.Hosts...)
	session, err := c.cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	err = session.Query(
		fmt.Sprintf(createKs, opts.KeySpace, opts.ReplicationFactor),
	).Exec()
	if err != nil {
		return nil, err
	}

	err = session.Query(
		fmt.Sprintf(createTable, opts.KeySpace, opts.TableName),
	).Exec()
	if err != nil {
		return nil, err
	}

	id := UUID()

	c.hosts = opts.Hosts
	c.keyspace = opts.KeySpace
	c.table = opts.TableName
	c.clientID = id
	c.Reconnect()

	return &c, nil
}

// Clone returns a copy of the currenct client
func (c *CassandraClient) Clone() Client {
	return &CassandraClient{
		cluster:     nil,
		hosts:       c.hosts,
		keyspace:    c.keyspace,
		table:       c.table,
		clientID:    c.clientID,
		session:     nil,
		consistency: c.consistency,
	}
}

// Reconnect reconnects to cassandra, or connects if not connected
func (c *CassandraClient) Reconnect() error {
	c.Close()
	c.cluster = gocql.NewCluster(c.hosts...)
	c.cluster.Keyspace = c.keyspace
	c.cluster.Consistency = c.consistency
	session, err := c.cluster.CreateSession()
	if err != nil {
		return err
	}
	c.session = session
	return nil
}

// SetID sets the ID for the current client
func (c *CassandraClient) SetID(id string) {
	c.clientID = id
}

// ID returns the current client ID
func (c *CassandraClient) ID() string {
	return c.clientID
}

// Close closes the connection to cassandra
func (c *CassandraClient) Close() {
	if c.session != nil {
		c.session.Close()
	}
}

// NewLock creates a new Lock. Lock is not automatically acquired.
func (c *CassandraClient) NewLock(name string) Lock {
	return &CassandraLock{
		name:   name,
		owner:  c.clientID,
		ttl:    time.Duration(0),
		client: c,
	}
}

// Acquire acquires the lock for the specified time lentgh (ttl).
// It returns immadiately if the lock cannot be acquired
func (l *CassandraLock) Acquire(ttl time.Duration) error {
	var name, owner, data string
	if ttl < time.Second {
		return ErrInvalidTTL
	}
	l.ttl = ttl
	query := fmt.Sprintf(acquireQ, l.client.keyspace, l.client.table, int(ttl.Seconds()))
	applied, err := l.client.session.Query(query, l.name, l.owner, l.data).ScanCAS(&name, &owner, &data)
	if err != nil {
		return err
	}
	if !applied && owner != l.owner {
		return ErrLockHeldByOtherClient
	}

	return nil
}

// Release releases the lock if owned. Returns an error if the lock is not owned by this client
func (l *CassandraLock) Release() error {
	var res string
	query := fmt.Sprintf(releaseQ, l.client.keyspace,
		l.client.table)
	applied, err := l.client.session.Query(query, l.name, l.owner).ScanCAS(&res)
	if err != nil {
		return err
	}
	if !applied {
		return ErrLockNotOwned
	}
	return nil
}

// Info returns information about the lock.
func (l *CassandraLock) Info() (*LockInfo, error) {
	var ttl int
	var owner, data string

	query := fmt.Sprintf(infoQ, l.client.keyspace, l.client.table)
	err := l.client.session.Query(query, l.name).SerialConsistency(gocql.Serial).Scan(&owner, &ttl, &data)
	if err == gocql.ErrNotFound {
		return &LockInfo{l.name, false, "", time.Duration(0), ""}, nil
	}
	if err != nil {
		return nil, err
	}
	return &LockInfo{
		Name:     l.name,
		Acquired: ttl > 0,
		Owner:    owner,
		TTL:      time.Duration(ttl) * time.Second,
		Data:     data,
	}, nil
}

// RefreshTTL Extends the lock, if owned, for the specified TTL.
// ttl argument becomes the new ttl for the lock: successive calls to Refresh()
// will use this ttl
// It returns an error if the lock is not owned by the current client
func (l *CassandraLock) RefreshTTL(ttl time.Duration) error {
	l.ttl = ttl
	return l.Refresh()
}

// Refresh extends the lock by extending the TTL in the store.
// It returns an error if the lock is not owned by the current client
func (l *CassandraLock) Refresh() error {
	var name string
	query := fmt.Sprintf(refreshQ, l.client.keyspace, l.client.table, int(l.ttl.Seconds()))
	applied, err := l.client.session.Query(query, l.owner, l.data, l.name, l.owner).ScanCAS(&name)
	if err != nil {
		return err
	}
	if !applied {
		return ErrLockNotOwned
	}
	return nil
}

// SetData sets the data payload for the lock.
// The data is set into the backend only when the lock is acquired,
// so any call to this method after acquisition won't update the value.
func (l *CassandraLock) SetData(data string) {
	l.data = data
}
