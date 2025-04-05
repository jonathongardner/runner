package runner

import (
	"fmt"
	"testing"
)

func (c *Controller) isFinished() bool {
	select {
	case <-c.finishChan:
		return true
	default:
		return false
	}
}

type foreverRunnner struct{}

func (f foreverRunnner) Run(rc *Controller) error {
	// This will run forever
	foo := make(chan bool)
	select {
	case <-foo:
	case <-rc.IsDone():
		return nil
	}
	return nil
}

type errorRunnner struct{}

func (f errorRunnner) Run(rc *Controller) error {
	return fmt.Errorf("foo chew foo")
}

type finishRunnner struct{}

func (f finishRunnner) Run(rc *Controller) error {
	return nil
}

func TestController(t *testing.T) {
	t.Run("Closes if nothing to wait on", func(t *testing.T) {
		c := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		c.GoBackground(foreverRunnner{})

		err := c.IsFinish()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be closed after creating")
		}
	})
	t.Run("Returns error if error returned", func(t *testing.T) {
		c := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		c.Go(errorRunnner{})
		c.Go(errorRunnner{})
		err := c.IsFinish()
		if err != ErrErrors {
			t.Errorf("expected no errors, got %v", err)
		}

		errStr := c.Errors()
		if errStr != "foo chew foo, foo chew foo" {
			t.Errorf("expected errors to be 'foo chew foo, foo chew foo', got %v", errStr)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
	})
	t.Run("Returns error if background error returned", func(t *testing.T) {
		c := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		c.GoBackground(errorRunnner{})
		c.Go(finishRunnner{})
		err := c.IsFinish()
		if err != ErrErrors {
			t.Errorf("expected no errors, got %v", err)
		}

		errStr := c.Errors()
		if errStr != "foo chew foo" {
			t.Errorf("expected errors to be 'foo chew foo', got %v", errStr)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
	})
}
