package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/gbagnoli/glock.v1"
)

func info(lock glock.Lock) {
	info, err := lock.Info()
	if err != nil {
		panic(err)
	}
	lockInfo(info)
}

func managerInfo(manager *glock.LockManager, name string) {
	info, err := manager.Info(name)
	if err != nil {
		panic(err)
	}
	lockInfo(info)
}

func lockInfo(i *glock.LockInfo) {
	fmt.Printf("Name: %s, Owner: %s, TTL: %s, Acquired: %v, Data: %s\n", i.Name, i.Owner, i.TTL, i.Acquired, i.Data)
}

func main() {
	tp := flag.String("type", "memory", "Driver to use: cassandra|redis|memory")
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
	case "memory":
		c = glock.NewMemoryClient("first")
		c2 = glock.NewMemoryClient("second")
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
	lock.SetData("My personal data")
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
		panic(err)
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
	manager := glock.NewLockManager(c, glock.AcquireOptions{TTL: 10 * time.Second})
	manager2 := glock.NewLockManager(c2, glock.AcquireOptions{TTL: 10 * time.Second})
	err = manager.Acquire("mylock", glock.AcquireOptions{MaxWait: 0})
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager2.Acquire("mylock", glock.AcquireOptions{MaxWait: 60 * time.Second, TTL: 10 * time.Second})
		fmt.Println("Acquired lock mylock in manager2")
		managerInfo(manager2, "mylock")
		time.Sleep(4 * time.Second)
		manager2.Release("mylock")
	}()
	managerInfo(manager, "mylock")
	manager.StartHeartbeat("mylock")
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		managerInfo(manager, "mylock")
	}
	manager.Release("mylock")
	wg.Wait()
}
