package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gbagnoli/glock"
)

func info(lock glock.Lock) {
	info, err := lock.Info()
	if err != nil {
		panic(err)
	}
	info_l(info)
}

func info_m(manager *glock.LockManager, name string) {
	info, err := manager.Info(name)
	if err != nil {
		panic(err)
	}
	info_l(info)
}

func info_l(i *glock.LockInfo) {
	fmt.Printf("Name: %s, Owner: %s, TTL: %s, Acquired: %s\n", i.Name, i.Owner, i.TTL, i.Acquired)
}

func main() {
	tp := flag.String("type", "cassandra", "Type of the driver: cassandra or redis")
	flag.Parse()
	var c, c2 glock.Client
	var err error

	switch *tp {
	case "cassandra":
		opts := glock.CassandraOptions{[]string{"localhost"}, "test", "", "", "test", 1}
		c, err = glock.NewCassandraLockClient(opts)
		c2, err = glock.NewCassandraLockClient(opts)
	case "redis":
		opts := glock.RedisOptions{"tcp", "localhost:6379", "", "myns", nil, nil}
		c, err = glock.NewRedisClient(opts)
		c2, err = glock.NewRedisClient(opts)
	default:
		fmt.Println("Invalid driver", *tp)
		os.Exit(2)
	}

	fmt.Println("Using driver", *tp)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	c.SetID("first")
	fmt.Printf("client id: %s\n", c.ID())
	lock := c.NewLock("mylock")
	info(lock)
	err = lock.Acquire(1800 * time.Second)
	if err != nil {
		panic(err)
	}
	c2.SetID("second")
	fmt.Printf("client id: %s\n", c2.ID())
	lock2 := c2.NewLock("mylock")
	err = lock2.Acquire(1800 * time.Second)
	if err == nil {
		panic("WUT")
	}
	err = lock.Refresh()
	if err != nil {
		panic(err)
	}
	info(lock)
	info(lock2)

	err = lock2.Release()
	if err == nil {
		panic(err)
	}

	err = lock.Release()
	if err != nil {
		panic(err)
	}
	lock2.Acquire(3500 * time.Millisecond)
	info(lock)
	lock2.Release()
	info(lock)

	fmt.Println("---------- Manager -------- ")
	manager := glock.NewLockManager(c, 10*time.Second)
	manager2 := glock.NewLockManager(c2, 10*time.Second)
	err = manager.TryAcquire("mylock")
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager2.Acquire("mylock", 60*time.Second)
		fmt.Println("Acquired lock mylock in manager2")
		info_m(manager2, "mylock")
		time.Sleep(4 * time.Second)
		manager2.Release("mylock")
	}()
	info_m(manager, "mylock")
	manager.StartHeartbeat("mylock")
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		info_m(manager, "mylock")
	}
	manager.Release("mylock")
	wg.Wait()
}
