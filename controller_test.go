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
	case <-rc.ShuttingDownChan():
		return nil
	}
	return nil
}

type runner struct {
	// used to wait for the controller to finish
	value    int
	prevWait chan bool
	wait     chan bool
	err      error
}

func newRunner(err error) *runner {
	prevWait := make(chan bool)
	close(prevWait)
	return &runner{
		value:    0,
		wait:     make(chan bool),
		prevWait: prevWait,
		err:      err,
	}
}

func (cw runner) newRunner(err error) *runner {
	return &runner{
		err:      err,
		value:    0,
		prevWait: cw.wait,
		wait:     make(chan bool),
	}
}

func (w *runner) Run(rc *Controller) error {
	select {
	case <-w.prevWait:
		select {
		case <-rc.ShuttingDownChan():
			w.value = 2
		default:
			w.value = 1
		}
	case <-rc.ShuttingDownChan():
		w.value = 2
	}
	close(w.wait)
	return w.err
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
		case <-rc.ShuttingDownChan():
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
		c, _ := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		c.Background(foreverRunnner{})

		if err := c.Wait(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be closed after creating")
		}
	})
	t.Run("Returns error if error returned but doesnt close", func(t *testing.T) {
		c, _ := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		w1 := newRunner(fmt.Errorf("foo"))
		c.Go(w1)
		w2 := w1.newRunner(fmt.Errorf("chew"))
		c.Go(w2)
		w3 := w2.newRunner(fmt.Errorf("bar"))
		c.Go(w3)

		if err := c.Wait(); err != ErrErrors {
			t.Errorf("expected no errors, got %v", err)
		}

		errStr := c.Errors()
		if errStr != "foo, chew, bar" {
			t.Errorf("expected errors to be 'foo, chew, bar', got %v", errStr)
		}

		if w2.value != 1 {
			t.Errorf("expected w3 to be 1, got %v", w3.value)
		}

		if w3.value != 1 {
			t.Errorf("expected w3 to be 1, got %v", w3.value)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
	})
	t.Run("Returns error if error returned and close", func(t *testing.T) {
		c, _ := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		// change to close on go errors
		c.CloseOnGoErrors()

		w1 := newRunner(fmt.Errorf("foo"))
		c.Go(w1)
		w2 := w1.newRunner(fmt.Errorf("chew"))
		c.Go(w2)
		w3 := w2.newRunner(fmt.Errorf("bar"))
		c.Go(w3)

		if err := c.Wait(); err != ErrErrors {
			t.Errorf("expected no errors, got %v", err)
		}

		errStr := c.Errors()
		if errStr != "foo" {
			t.Errorf("expected errors to be 'foo', got %v", errStr)
		}

		if w2.value != 2 {
			t.Errorf("expected w3 to be 1, got %v", w3.value)
		}

		if w3.value != 2 {
			t.Errorf("expected w3 to be 1, got %v", w3.value)
		}

		if !c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
	})
	t.Run("Returns error if background error returned", func(t *testing.T) {
		c, _ := NewController()
		if c.isFinished() {
			t.Errorf("expected finishChan to be open after creating")
		}
		c.Background(newRunner(fmt.Errorf("foo chew foo")))
		c.Go(newRunner(nil))
		if err := c.Wait(); err != ErrErrors {
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
	t.Run("returns error if bad limit", func(t *testing.T) {
		_, err := NewControllerWithLimit(0)
		if err != ErrInvalidLimit {
			t.Errorf("expected invalid limit error, got %v", err)
		}
	})
	t.Run("Go is not limited", func(t *testing.T) {
		c, err := NewControllerWithLimit(1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
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
		if err := c.Wait(); err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{1, 2, 3, 4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
	t.Run("LimitedGo and Go", func(t *testing.T) {
		c, _ := NewControllerWithLimit(1)
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
		if err := c.Wait(); err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{1, 2, 3, 4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
	t.Run("LimitedGo is... limited", func(t *testing.T) {
		c, _ := NewControllerWithLimit(1)
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
		if err := c.Wait(); err != nil {
			t.Errorf("expected no errors, got %v", err)
		}
		if !rec.match([]int{4}) {
			t.Errorf("expected reciever to match, got %v", rec.received)
		}
	})
}
