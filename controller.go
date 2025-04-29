package runner

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var ErrErrors = fmt.Errorf("error running the jobs")
var ErrInvalidLimit = fmt.Errorf("limit must be greater than 0")
var ErrShuttingDown = fmt.Errorf("shutting down")

// The default limit for the limit controller
var defaultLimit = 4

// SetDefaultLimit change the defualt limit
func SetDefaultLimit(limit int) {
	defaultLimit = limit
}

// Runner is an interface that defines a method to run a job.
type Runner interface {
	Run(rc *Controller) error
}

// Controller can run two types of jobs:
// - Runner: When these finish `Done()` will be called
// - BackgroundRunner: These should listen to `IsDone()` and gracefully exit
type Controller struct {
	//----running------
	mainCountChan  chan bool   // true up false down
	backCountChan  chan bool   // true up false down
	limitCountChan chan bool   // true up false down
	limiter        chan (bool) // used to limit the number of concurrent jobs
	//----listeners------
	doneChan   chan struct{} // used for `Done` (notify other of gracefully close)
	finishChan chan struct{} // used for `Wait` (notify main of finished)
	//-----Pass errors-----
	errorChan chan error
	errors    []string
	//-----Order Restorer------
	or           *OrderRestorer
	closeOnError bool
}

// NewController returns a new controller with default values
func NewController() (*Controller, error) {
	return NewControllerWithLimit(defaultLimit)
}

// NewControllerWithLimit returns a new controller with with a variable limit size
func NewControllerWithLimit(limit int) (*Controller, error) {
	if limit < 1 {
		return nil, ErrInvalidLimit
	}
	dc := make(chan struct{})
	c := &Controller{
		mainCountChan:  make(chan bool),
		backCountChan:  make(chan bool),
		limitCountChan: make(chan bool),
		limiter:        make(chan bool, limit),
		doneChan:       dc,
		finishChan:     make(chan struct{}),
		errorChan:      make(chan error),
		errors:         make([]string, 0),
		closeOnError:   false,
		or:             NewOrderRestorer(dc),
	}

	go c.listenForCtrlC()
	go c.runMain()
	go c.runErr()

	return c, nil
}

func (c *Controller) CloseOnGoErrors() {
	c.closeOnError = true
}

//----------------Handle close-----------------

// listenForCtrlC listens for Ctrl+C and gracefully shuts down the controller
func (c *Controller) listenForCtrlC() {
	ch := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	<-ch
	log.Info("Gracefully shutting down...")
	c.Shutdown()

	<-ch
	log.Info("Killing!")
	c.Finish()
}

// Shutdown used to gracefully close everything
func (c *Controller) Shutdown() {
	log.Debug("Done")
	select {
	case <-c.doneChan:
		log.Debug("Already shuting down...")
	default:
		close(c.doneChan)
	}
}

// ShuttingDownChan will return a channel that will be closed when the controller is shutting down
func (c *Controller) ShuttingDownChan() <-chan struct{} {
	return c.doneChan
}

// IsShuttingDown will return true if the controller is shutting down
func (c *Controller) IsShuttingDown() bool {
	select {
	case <-c.ShuttingDownChan():
		return true
	default:
		return false
	}
}

// IsShuttingDown will return true if the controller is shutting down
func (c *Controller) ShuttingDownErr() error {
	if c.IsShuttingDown() {
		return ErrShuttingDown
	}
	return nil

}

// Finish used to exit (should call Shutdown to gracefully close everything)
func (c *Controller) Finish() {
	log.Debug("Finish")
	select {
	case <-c.finishChan:
		log.Debug("Already finished...")
	default:
		close(c.finishChan)
	}
}

// Wait will wait until all jobs are finished
func (c *Controller) Wait() error {
	c.mainCountChan <- false  // so that we dont close until something is waiting
	c.limitCountChan <- false // so that we dont close until something is waiting
	<-c.finishChan
	if len(c.errors) == 0 {
		return nil
	}

	return ErrErrors
}

func (c *Controller) Errors() string {
	if len(c.errors) == 0 {
		return ""
	}

	return strings.Join(c.errors, ", ")
}

// NextOR returns an OrderRestorer and queues up the next order restorer
func (c *Controller) NextOR() *OrderRestorer {
	toReturn := c.or
	c.or = c.or.Next()
	return toReturn
}

//----------------Handle close-----------------

func (c *Controller) runMain() {
	mainCount := 1  // start with 1 so dont try closing until wait is called
	limitCount := 1 // start with 1 so dont try closing until wait is called
	backgroundCount := 0

	for {
		select {
		case mc := <-c.mainCountChan:
			if mc {
				mainCount += 1
			} else {
				mainCount -= 1
			}
		case bc := <-c.backCountChan:
			if bc {
				backgroundCount += 1
			} else {
				backgroundCount -= 1
			}
		case bc := <-c.limitCountChan:
			if bc {
				limitCount += 1
			} else {
				limitCount -= 1
			}
		}

		if mainCount == 0 && limitCount == 0 {
			c.Shutdown()
		}

		log.Debugf("Main %v, Limmited %v, Background %v", mainCount, limitCount, backgroundCount)
		if mainCount+limitCount+backgroundCount == 0 {
			select {
			case <-c.doneChan:
				log.Debug("Closing Error Chan")
				close(c.errorChan)
			default:
				log.Debug("Not done?")
			}
		}
	}
}

func (c *Controller) runErr() {
	for {
		newError, ok := <-c.errorChan
		if !ok {
			break
		}
		c.errors = append(c.errors, newError.Error())
	}
	c.Finish()
}
