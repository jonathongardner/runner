package runner

import "fmt"

var ErrControllerFinished = fmt.Errorf("already finished")

type OrderRestorer struct {
	done chan struct{}
	prev chan struct{}
	next chan struct{}
}

func NewOrderRestorer(done chan struct{}) *OrderRestorer {
	prev := make(chan struct{})
	close(prev)
	return &OrderRestorer{
		done: done,
		prev: prev,
		next: make(chan struct{}),
	}
}

func (o *OrderRestorer) Next() *OrderRestorer {
	return &OrderRestorer{
		next: make(chan struct{}),
		done: o.done,
		prev: o.next,
	}
}

func (o *OrderRestorer) Finished() {
	close(o.next)
}

func (o *OrderRestorer) Wait() error {
	// Wait for the previous channel to be closed
	select {
	case <-o.prev:
		return nil
	case <-o.done:
		return ErrControllerFinished
	}
}
