package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gbagnoli/glock"
)

type cHostsFlag struct {
	hosts []string
}

func (f *cHostsFlag) Hosts() []string {
	if len(f.hosts) == 0 {
		return []string{"localhost"}
	}
	return f.hosts
}

func (f *cHostsFlag) String() string {
	return fmt.Sprint(f.hosts)
}

func (f *cHostsFlag) Set(value string) error {
	if len(f.hosts) > 0 {
		return errors.New("cassandra hosts are already set")
	}

	hosts := strings.Split(value, ",")
	for _, elem := range hosts {
		f.hosts = append(f.hosts, elem)
	}
	return nil
}

var driver = flag.String("driver", "cassandra", "driver to use")
var name = flag.String("lock", "", "lock name. Required")
var id = flag.String("client-id", "", "if unset, it will be autogenerated")
var ttl = flag.Int("lock-ttl-seconds", 30, "TTL for locks, in seconds")
var quiet = flag.Bool("quiet", false, "Disable logging in glock")

var redisAddress = flag.String("redis-server", "localhost:6379", "redis server address (with port)")
var redisNS = flag.String("redis-namspace", "glock", "namespace for keys in redis. Default is used even if set to be empty on commandline")

var cHosts cHostsFlag
var cassandraKS = flag.String("cassandra-ks", "glock", "cassandra keyspace")
var cassandraTable = flag.String("cassandra-table", "glock", "cassandra table")
var cassandraUsername = flag.String("cassandra-username", "", "cassandra username")
var cassandraPassword = flag.String("cassandra-password", "", "cassandra password")
var cassandraReplFactor = flag.Int("cassandra-replication-factor", 1, "Cassandra replication factor (only used if ks needs to be created")

func exit(err error) {
	if err == nil {
		os.Exit(0)
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// Process exited with returncode != 0
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			os.Exit(status.ExitStatus())
		} else {
			log.Fatalf("cannot get the return code of the process: %v", err)
		}
	} else {
		log.Fatalf("error calling wait() on subprocess: %v", err)
	}
}

func kill(command *exec.Cmd) {
	if err := command.Process.Kill(); err != nil {
		log.Fatalf("Error while killing process: %s", err.Error())
	}
	command.Process.Signal(syscall.SIGKILL)
}

func fwdSignal(command *exec.Cmd, sig os.Signal) {
	if err := command.Process.Signal(sig); err != nil {
		log.Printf("Error while sending signal %s: %s", sig, err.Error())
	}
	if sig == syscall.SIGTSTP {
		log.Print("glock: SIGTSTP sent to child process, but will continue to send heartbeats for the locks")
	}
}

func main() {
	flag.Var(&cHosts, "cassandra-hosts", "Comma separated list of cassandra hosts")
	flag.Parse()

	var client glock.Client
	var err error

	switch *driver {
	case "cassandra":
		opts := glock.CassandraOptions{
			Hosts:             cHosts.Hosts(),
			KeySpace:          *cassandraKS,
			TableName:         *cassandraTable,
			Username:          *cassandraUsername,
			Password:          *cassandraPassword,
			ReplicationFactor: *cassandraReplFactor,
		}
		client, err = glock.NewCassandraLockClient(opts)

	case "redis":
		opts := glock.RedisOptions{
			Network:   "tcp",
			Address:   *redisAddress,
			Namespace: *redisNS,
		}
		client, err = glock.NewRedisClient(opts)

	default:
		log.Fatalf("Invalid value for --driver '%s'", *driver)
	}

	if err != nil {
		log.Fatalf("Cannot create lock client: %s", err.Error())
	}

	if *name == "" {
		log.Print("Missing lock name (required)")
		flag.Usage()
		os.Exit(1)
	}

	if *id != "" {
		client.SetID(*id)
	}
	manager := glock.NewLockManager(client, time.Duration(*ttl)*time.Second)
	if !*quiet {
		manager.Logger.SetOutput(os.Stderr)
	}

	manager.Logger.Printf("Using driver: %s", *driver)
	err = manager.TryAcquire(*name)
	if err != nil {
		manager.Logger.Fatalf("%s/%s: Cannot acquire lock '%s': %s", *driver, client.ID(), *name, err.Error())
	}
	defer func() {
		err = manager.Release(*name)
		if err != nil {
			manager.Logger.Printf("Could not release lock '%s': %s", *name, err.Error())
		}
	}()

	args := flag.Args()
	commandStr := strings.Join(args, " ")
	manager.Logger.Printf("Running: %s", commandStr)
	command := exec.Command(args[0], args[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err = command.Start()
	if err != nil {
		log.Printf("Cannot start command %s: %s", commandStr, err.Error())
		return
	}

	control, err := manager.StartHeartbeat(*name)
	if err != nil {
		log.Printf("%s/%s: Cannot start heartbeats for lock '%s': %s", *driver, client.ID(), *name, err.Error())
		kill(command)
	}

	done := make(chan error, 1)
	ksigs := make(chan os.Signal, 1)
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs)
	signal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGCHLD)
	signal.Notify(ksigs, os.Interrupt, syscall.SIGTERM)

	go func() {
		done <- command.Wait()
	}()

	for {
		select {
		case <-control:
			log.Printf("Cannot refresh lock, killing process")
			kill(command)
		case sig := <-sigs:
			manager.Logger.Printf("Forwarding signal %s to child", sig)
			fwdSignal(command, sig)
		case sig := <-ksigs:
			manager.Logger.Printf("Received signal '%s', killing process", sig)
			kill(command)
		case err = <-done:
			manager.ReleaseAll()
			exit(err)
		}
	}
}