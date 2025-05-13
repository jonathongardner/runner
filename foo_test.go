package runner

import (
	"context"
	"testing"
)

//--------------Reciever Sender Runners-----------------

func TestContext(t *testing.T) {
	c1 := context.Background()
	select {
	case <-c1.Done():
		t.Error("Expected c1 not to be done")
	default:
	}
	c2, canc := context.WithCancel(c1)
	select {
	case <-c2.Done():
		t.Error("Expected c2 not to be done")
	default:
	}
	canc()
	select {
	case <-c2.Done():
	default:
		t.Error("Expected c2 to be done")
	}

	select {
	case <-c1.Done():
		t.Error("Expected c1 not to be done though c2 is")
	default:
	}

	if c2.Err() == nil {
	        t.Error("Expected err for c2 but got no err")
	}

	if c1.Err() == nil {
	        t.Error("Expected err for c1 but got no err")
	}
}
