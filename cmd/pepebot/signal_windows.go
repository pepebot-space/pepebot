//go:build windows

package main

import "os"

// restartSignal is a custom signal type for Windows restart support
// since SIGHUP is not available on Windows.
type restartSignal struct{}

func (restartSignal) Signal() {}
func (restartSignal) String() string { return "restart" }

var restartChan chan<- os.Signal

func notifyRestartSignal(sigChan chan<- os.Signal) {
	restartChan = sigChan
}

func triggerRestart() {
	if restartChan != nil {
		restartChan <- restartSignal{}
	}
}

func isRestartSignal(sig os.Signal) bool {
	_, ok := sig.(restartSignal)
	return ok
}
