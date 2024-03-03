package cleanup

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
)

const (
	Redis = iota
	Echo
)

type OnStop func(sig os.Signal)

// stop is a struct that represents a stop instance
type stop struct {
	isStopping bool           // isStopping is false by default
	mutex      sync.Mutex     // mutex for the global stop instance
	onStopFunc map[int]OnStop // map of functions to call when stopping
}

// global instance of stop
var quitInstance = &stop{
	isStopping: false,                // isStopping is false by default
	onStopFunc: make(map[int]OnStop), // initializes the onStopFunc map
}

// AddOnStopFunc adds a function to the onStopFunc map
//   - adds the function to the onStopFunc map
//   - checks if the stop instance has the isStopping flag set to true
//   - calls the function
//
// @param key int - the key for the onStopFunc map
// @param f OnStop - the function to add to the onStopFunc map
func AddOnStopFunc(key int, f OnStop) {
	// locks the mutex for the global stop instance
	quitInstance.mutex.Lock()
	// unlocks the mutex after the function returns
	defer quitInstance.mutex.Unlock()
	// adds the function to the onStopFunc map
	quitInstance.onStopFunc[key] = f
	// checks if the stop instance has the isStopping flag set to true
	if quitInstance.isStopping {
		// calls the function
		f(syscall.SIGTERM)
	}
}

// Stops the function instance
//   - sets isStopping to true
//   - calls the functions in the onStopFunc map
//   - deletes the function from the onStopFunc map
//
// @param sig os.Signal - the signal to stop the function instance
func Stop(sig os.Signal) {
	// locks the mutex for the global stop instance
	quitInstance.mutex.Lock()
	// unlocks the mutex after the function returns
	defer quitInstance.mutex.Unlock()
	// sets isStopping to true
	quitInstance.isStopping = true
	// logs the signal
	log.Warnf("Received signal %d, terminating...", sig)
	// calls the functions in the onStopFunc map
	for k, f := range quitInstance.onStopFunc {
		f(sig)
		// deletes the function from the onStopFunc map
		delete(quitInstance.onStopFunc, k)
	}
}

// Runs the function with the given key
//   - iterates through the keys to run the function with the given key for the onStopFunc map
//   - deletes the function from the onStopFunc map
//
// @param sig os.Signal - the signal to stop the function instance
// @param keys ...int - the keys for the onStopFunc map
func RunStopFunc(sig os.Signal, keys ...int) {
	// locks the mutex for the global stop instance
	quitInstance.mutex.Lock()
	// unlocks the mutex after the function returns
	defer quitInstance.mutex.Unlock()
	// iterates through the keys to run the function with the given key for the onStopFunc map
	for _, key := range keys {
		if f, ok := quitInstance.onStopFunc[key]; ok {
			f(sig)
			// deletes the function from the onStopFunc map
			delete(quitInstance.onStopFunc, key)
		}
	}
}

// Stops all except the given keys - **Deprecated**
//   - sets isStopping to true
//   - creates a set of the except keys
//   - iterates through the keys to run the function with the given key for the onStopFunc map
//   - deletes the function from the onStopFunc map if the key is not in the except set
//
// @param sig os.Signal - the signal to stop the function instance
// @param except ...int - the keys to not stop
func StopAllExcept(sig os.Signal, except ...int) {
	quitInstance.mutex.Lock()
	defer quitInstance.mutex.Unlock()
	quitInstance.isStopping = true
	log.Warnf("Stopping all except %v", except)
	// creates a set of the except keys
	exceptSet := mapset.NewSet[int](except...)
	// iterates through the keys to run the function with the given key for the onStopFunc map
	for k, f := range quitInstance.onStopFunc {
		// deletes the function from the onStopFunc map if the key is not in the except set
		if !exceptSet.Contains(k) {
			f(sig)
			delete(quitInstance.onStopFunc, k)
		}
	}
}

// InitSignalCallback initializes the signal callback+
//   - creates a channel for the signal
//   - registers the signal channel
//   - calls the Stop function when the signal is received on the channel
func InitSignalCallback() {
	// creates a channel for the signal
	sigChan := make(chan os.Signal, 1)
	// registers the signal channel
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	// calls the Stop function when the signal is received on the channel
	go func() {
		sig := <-sigChan
		// calls the Stop function
		Stop(sig)
	}()
}
