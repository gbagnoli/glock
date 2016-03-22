package glock

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Exec executes the command only if the lock can be acquired
// It refreshes the lock using manager's heartbeats, and terminates the command
// if the lock is lost somehow.
// It will wait up to maxWait for the lock to be acquired
// returns the return code of the command, and any errors
func (manager *LockManager) Exec(lock string, command *exec.Cmd, maxWait time.Duration) (int, error) {
	return manager.exec(lock, command, maxWait, time.Duration(-1))
}

// ExecTTL is like Exec but an explicit TTL can be set for the lock
func (manager *LockManager) ExecTTL(lock string, command *exec.Cmd, maxWait, ttl time.Duration) (int, error) {
	return manager.exec(lock, command, maxWait, ttl)
}

// TryExec is like Exec, except it will fail without waiting if the lock cannot
// be acquired
func (manager *LockManager) TryExec(lock string, command *exec.Cmd) (int, error) {
	return manager.exec(lock, command, time.Duration(-1), time.Duration(-1))
}

// TryExecTTL is like TryExec, but an explicit TTL can be set for the lock
func (manager *LockManager) TryExecTTL(lock string, command *exec.Cmd, ttl time.Duration) (int, error) {
	return manager.exec(lock, command, time.Duration(-1), ttl)
}

func (manager *LockManager) exec(lock string, command *exec.Cmd, maxWait, ttl time.Duration) (int, error) {
	var err error
	client := manager.Client()
	if maxWait < 0 {
		if ttl < 0 {
			err = manager.TryAcquire(lock)
		} else {
			err = manager.TryAcquireTTL(lock, ttl)
		}
	} else {
		if ttl < 0 {
			err = manager.Acquire(lock, maxWait)
		} else {
			err = manager.AcquireTTL(lock, maxWait, ttl)
		}
	}

	if err != nil {
		manager.Logger.Printf("Exec (%s); Cannot acquire lock '%s': %s", client.ID(), lock, err.Error())
		return -1, err
	}

	defer func() {
		err = manager.Release(lock)
		if err != nil {
			manager.Logger.Printf("Exec (%s); Cannot release lock '%s': %s", client.ID(), lock, err.Error())
		}
	}()

	commandStr := strings.Join(command.Args, " ")
	err = command.Start()
	if err != nil {
		manager.Logger.Printf("Exec (%s); Cannot start command '%s': %s", client.ID(), commandStr, err.Error())
		return -1, err
	}

	control, err := manager.StartHeartbeat(lock)
	if err != nil {
		manager.Logger.Printf("Exec (%s); Cannot start heartbeats for lock '%s': %s", client.ID(), lock, err.Error())
		kill(command)
		return -1, err
	}

	done := make(chan error, 1)
	ksigs := make(chan os.Signal, 1)
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs)
	defer signal.Stop(sigs)
	signal.Notify(ksigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ksigs)

	go func() {
		done <- command.Wait()
	}()

	for {
		select {
		case sig := <-sigs:
			manager.Logger.Printf("Forwarding signal %s to child", sig)
			fwdSignal(manager.Logger, command, sig)

		case sig := <-ksigs:
			manager.Logger.Printf("Received signal '%s', killing process", sig)
			kill(command)
			// do not return here, it will arrive a message on the done channel

		case refreshError := <-control:
			manager.Logger.Printf("Exec(%s); Cannot refresh lock '%s', killing process: %s", client.ID(), lock, refreshError.Error())
			kill(command)
			return -1, refreshError

		case err = <-done:
			return exit(err)
		}
	}
}

func exit(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		// Process exited with returncode != 0
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), nil
		}

		log.Printf("cannot get the return code of the process: %v", err)
		return -1, err
	}

	log.Printf("error calling wait() on subprocess: %v", err)
	return -1, err
}

func kill(command *exec.Cmd) {
	if err := command.Process.Kill(); err != nil {
		log.Printf("Error while killing process: %s", err.Error())
	}
	// FIXME we should check if the process is still alive before sending SIGKILL
	command.Process.Signal(syscall.SIGKILL)
}

func fwdSignal(logger *log.Logger, command *exec.Cmd, sig os.Signal) {
	switch sig {
	case os.Interrupt, syscall.SIGTERM, syscall.SIGCHLD:
		return
	}
	if err := command.Process.Signal(sig); err != nil {
		logger.Printf("glock: Error while sending signal %s: %s", sig, err.Error())
	}
	if sig == syscall.SIGTSTP {
		logger.Print("glock: SIGTSTP sent to child process, but will continue to send heartbeats for the locks")
	}
}
