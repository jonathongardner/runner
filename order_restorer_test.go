package runner

import (
	"slices"
	"testing"
)

//--------------Reciever Sender Runners-----------------

func TestOrderRestorer(t *testing.T) {
	done := make(chan struct{})
	t1 := NewOrderRestorer(done)
	t2 := t1.Next()
	t3 := t2.Next()
	t4 := t3.Next()
	t5 := t4.Next()
	t6 := t5.Next()

	vals := []int{}

	go func() {
		t1.Wait()
		vals = append(vals, 1)
		t1.Finished()

		t3.Wait()
		vals = append(vals, 3)
		t3.Finished()

		t5.Wait()
		vals = append(vals, 5)
		t5.Finished()
	}()

	// just confirm that one doesnt return an error, assume others dont...
	if err := t2.Wait(); err != nil {
		t.Errorf("Expected nil but got %v", err)
	}
	vals = append(vals, 2)
	t2.Finished()

	t4.Wait()
	vals = append(vals, 4)
	t4.Finished()

	t6.Wait()
	vals = append(vals, 6)
	t6.Finished()

	if !slices.Equal(vals, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("Expected [1 2 3 4 5 6] but got %v", vals)
	}
}

//--------------Reciever Sender Runners-----------------

func TestWhenLaterFinishedCalledOrderRestroererStillWorks(t *testing.T) {
	done := make(chan struct{})
	t1 := NewOrderRestorer(done)
	t2 := t1.Next()
	tm := t2.Next()
	t3 := tm.Next()

	vals := []int{}

	go func() {
		// just confirm that one doesnt return an error, assume others dont...
		t2.Wait()
		vals = append(vals, 2)
		t2.Finished()
	}()

	go func() {
		t1.Wait()
		vals = append(vals, 1)
		t1.Finished()
	}()

	tm.Finished()

	t3.Wait()
	vals = append(vals, 3)
	t3.Finished()

	if !slices.Equal(vals, []int{1, 2, 3}) {
		t.Errorf("Expected [1 2 3] but got %v", vals)
	}
}

func TestAlreadyFinishedOrderRestorer(t *testing.T) {
	done := make(chan struct{})
	close(done)
	t1 := NewOrderRestorer(done)
	t2 := t1.Next()

	// Cant wait on t1 becasue prev is already closed
	// so select between the two is not deterministic
	// or maybe it picks the first, idk, either way
	// t2 guarantees that we check the done channel

	if err := t2.Wait(); err != ErrControllerFinished {
		t.Errorf("Expected %v but got %v", ErrControllerFinished, err)
	}
}
