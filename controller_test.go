package runner

import (
	"fmt"
	"slices"
	"sort"
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

// --------------Reciever Sender Runners-----------------
// RecieverRunnner is a test runner that receives values from a channel
// and stores them in a slice. It returns an error if the channel is closed
type recieverRunnner struct {
	pass     chan int
	mess     chan bool
	received []int
	num      int
}

func newRecieverRunnner(size int) *recieverRunnner {
	return &recieverRunnner{
		pass:     make(chan int, size),
		mess:     make(chan bool),
		received: []int{},
		num:      size,
	}
}

func (rr *recieverRunnner) Run(rc *Controller) error {
	for {
		select {
		case i := <-rr.mess:
			if i {
				return nil
			}
		case i := <-rr.pass:
			rr.received = append(rr.received, i)
			rr.num--
			if rr.num == 0 {
				return nil
			}
		case <-rc.IsDone():
			return fmt.Errorf("closed receiver early")
		}
	}
}

// used to check if the receiver is started
func (rr *recieverRunnner) started() {
	rr.mess <- false
}

// used to stop the receiver
func (rr *recieverRunnner) stop() {
	rr.mess <- true
}
func (rr *recieverRunnner) add(i int) {
	rr.pass <- i
}
func (rr *recieverRunnner) match(b []int) bool {
	sort.Ints(rr.received)
	return slices.Equal(rr.received, b)
}

type senderRunnner struct {
	value int
	rec   *recieverRunnner
}

func newSenderRunnner(value int, rec *recieverRunnner) *senderRunnner {
	return &senderRunnner{
		value: value,
		rec:   rec,
	}
}
func (sr *senderRunnner) Run(rc *Controller) error {
	sr.rec.add(sr.value)
	return nil
}

//--------------Reciever Sender Runners-----------------

func TestController(t *testing.T) {
	// SetLogger(fmtLogger{})

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
			t.Errorf("expected ErrErrors, got %v", err)
		}

		errStr := c.Errors()
		if errStr != "foo chew foo" {
			t.Errorf("expected errors to be 'foo chew foo', got %v", errStr)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
	})
	t.Run("Go is not limited", func(t *testing.T) {
		c := NewControllerWithLimit(1)
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		rec := newRecieverRunnner(4)
		c.Go(rec)

		senders := []*senderRunnner{
			newSenderRunnner(1, rec),
			newSenderRunnner(2, rec),
			newSenderRunnner(3, rec),
		}

		// Need to wait for rec to start b/c the go routines arent in order
		// so want to make sure it started
		rec.started()
		for _, s := range senders {
			c.Go(s)
		}
		// just add one to check
		rec.add(4)
		err := c.IsFinish()
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{1, 2, 3, 4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
	t.Run("LimitedGo and Go", func(t *testing.T) {
		c := NewControllerWithLimit(1)
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		rec := newRecieverRunnner(4)
		c.LimitedGo(rec)

		senders := []*senderRunnner{
			newSenderRunnner(1, rec),
			newSenderRunnner(2, rec),
			newSenderRunnner(3, rec),
		}

		// Need to wait for rec to start b/c the go routines arent in order
		// so want to make sure it started
		rec.started()
		for _, s := range senders {
			c.Go(s)
		}
		rec.add(4)
		err := c.IsFinish()
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{1, 2, 3, 4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
	t.Run("LimitedGo is... limited", func(t *testing.T) {
		c := NewControllerWithLimit(1)
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		rec := newRecieverRunnner(4)
		c.LimitedGo(rec)

		senders := []*senderRunnner{
			newSenderRunnner(1, rec),
			newSenderRunnner(2, rec),
			newSenderRunnner(3, rec),
		}

		// Need to wait for rec to start b/c the go routines arent in order
		// so want to make sure it started
		rec.started()
		for _, s := range senders {
			c.LimitedGo(s)
		}
		rec.add(4)
		rec.stop()
		err := c.IsFinish()
		if err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
}
