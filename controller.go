package runner

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var ErrErrors = fmt.Errorf("error running the jobs")

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
}

// NewController returns a new controller with default values
func NewController() *Controller {
	return NewControllerWithLimit(defaultLimit)
}

// NewControllerWithLimit returns a new controller with with a variable limit size
func NewControllerWithLimit(limit int) *Controller {
	c := &Controller{
		mainCountChan:  make(chan bool),
		backCountChan:  make(chan bool),
		limitCountChan: make(chan bool),
		limiter:        make(chan bool, limit),
		doneChan:       make(chan struct{}),
		finishChan:     make(chan struct{}),
		errorChan:      make(chan error),
		errors:         make([]string, 0),
	}

	go c.listenForCtrlC()
	go c.runMain()
	go c.runErr()

	return c
}

//----------------Handle close-----------------

// listenForCtrlC listens for Ctrl+C and gracefully shuts down the controller
func (c *Controller) listenForCtrlC() {
	ch := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	<-ch
	log.Info("Gracefully shutting down...")
	c.Done()

	<-ch
	log.Info("Killing!")
	c.Finish()
}

// Done used to gracefully close everything
func (c *Controller) Done() {
	log.Debug("Done")
	select {
	case <-c.doneChan:
		log.Debug("Already shuting down...")
	default:
		close(c.doneChan)
	}
}
func (c *Controller) IsDone() chan struct{} {
	return c.doneChan
}

// Finish used to exit
func (c *Controller) Finish() {
	log.Debug("Finish")
	select {
	case <-c.finishChan:
		log.Debug("Already finished...")
	default:
		close(c.finishChan)
	}
}

// IsFinish will wait until all jobs are finished
func (c *Controller) IsFinish() error {
	c.mainCountChan <- false // so that we dont close until something is waiting
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

//----------------Handle close-----------------

func (c *Controller) runMain() {
	mainCount := 1 // start with 1 so dont try closing until wait is called
	backgroundCount := 0
	limitCount := 0

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
			c.Done()
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

// Go run all of these until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()`
func (c *Controller) Go(runner Runner) {
	select {
	case <-c.doneChan:
		log.Debug("Not running job because shuting down")
	default:
		c.mainCountChan <- true
		go func() {
			err := runner.Run(c)
			if err != nil {
				c.errorChan <- err
			}
			c.mainCountChan <- false
		}()
	}
}

// LimitedGo run all of these with a limit until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()`
func (c *Controller) LimitedGo(runner Runner) {
	select {
	case <-c.doneChan:
		log.Debug("Not running limited job because shuting down")
	case c.limiter <- true:
		c.limitCountChan <- true
		go func() {
			err := runner.Run(c)
			if err != nil {
				c.errorChan <- err
			}
			c.limitCountChan <- false
			select {
			case <-c.limiter:
			default:
				log.Errorf("no more limiter available")
			}
		}()
	}
}

// Running in background will get stopped when all `Go` created ones finish
// if the bgRunner returns error it will gracefully shutdown everything else
func (c *Controller) GoBackground(bgRunner Runner) {
	select {
	case <-c.doneChan:
		log.Debug("Not running background job because shuting down")
	default:
		c.backCountChan <- true
		go func() {
			err := bgRunner.Run(c)
			if err != nil {
				c.errorChan <- err
				c.Done()
			}
			c.backCountChan <- false
		}()
	}
}
