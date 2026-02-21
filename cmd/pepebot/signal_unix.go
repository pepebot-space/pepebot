//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyRestartSignal(sigChan chan<- os.Signal) {
	signal.Notify(sigChan, syscall.SIGHUP)
}

func triggerRestart() {
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
}

func isRestartSignal(sig os.Signal) bool {
	return sig == syscall.SIGHUP
}
