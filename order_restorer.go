package runner

import "fmt"

var ErrControllerFinished = fmt.Errorf("already finished")

type OrderRestorer struct {
	done chan struct{}
	prev chan struct{}
	next chan struct{}
}

// NewOrderRestorer returns a new OrderRestorer
// expects a channel that will be closed that can be used to exit
func NewOrderRestorer(done chan struct{}) *OrderRestorer {
	prev := make(chan struct{})
	close(prev)
	return &OrderRestorer{
		done: done,
		prev: prev,
		next: make(chan struct{}),
	}
}

// Next will return a new OrderRestorer that is next in line
func (o *OrderRestorer) Next() *OrderRestorer {
	return &OrderRestorer{
		next: make(chan struct{}),
		done: o.done,
		prev: o.next,
	}
}

// Finished close the channel of the next OR so it can be used
// will wait for the previous channel to be closed if it has not
func (o *OrderRestorer) Finished() error {
	err := o.Wait()
	close(o.next)
	return err
}

// Wait will wait for the previous channel to be closed
// and return an error if the controller is finished
func (o *OrderRestorer) Wait() error {
	// Wait for the previous channel to be closed
	select {
	case <-o.prev:
		return nil
	case <-o.done:
		return ErrControllerFinished
	}
}
