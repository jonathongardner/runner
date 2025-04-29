package runner

func (c *Controller) addCount(v chan bool, function func(callback func())) {
	select {
	case <-c.doneChan:
		log.Debug("Not adding count b/c shuting down")
	case v <- true:
		function(func() {
			v <- false
		})
	}
}

// waitForLimiter waits until c.limit chan has room. If shutdown before, it will
// skip running the function and return true (false means it ran the function)
func (c *Controller) waitForLimiter(function func()) bool {
	select {
	case <-c.doneChan:
		log.Debug("Not running limited job because shuting down")
		return true
	case c.limiter <- true:
		log.Debug("Got limiter")
		function()
		return false
	}
}
func (c *Controller) releaseLimiter() {
	select {
	case <-c.limiter:
	default:
		log.Errorf("no more limiter available")
	}
}

// Go run all of these until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()` It will continue
// to run unlses CloseOnGoError is set to true
func (c *Controller) Go(runner Runner) {
	c.addCount(c.mainCountChan, func(finished func()) {
		go func() {
			defer finished()
			c.addError(runner.Run(c))
		}()
	})
}

// BGo Same as `Go` but run in the current thread
func (c *Controller) BGo(runner Runner) {
	c.addCount(c.mainCountChan, func(finished func()) {
		defer finished()
		c.addError(runner.Run(c))
	})
}

// LimitedGo run all of these with a limit until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()`. It will continue
// to run unlses CloseOnGoError is set to true
func (c *Controller) LimitedGo(runner Runner) {
	c.addCount(c.limitCountChan, func(finished func()) {
		go func() {
			defer finished()
			c.waitForLimiter(func() {
				c.addError(runner.Run(c))
				c.releaseLimiter()
			})
		}()
	})
}

// BlLimitedGo is the same as LimitedGo but it will block adding
// to the limiter until one is free
func (c *Controller) BlLimitedGo(runner Runner) {
	// need to add count first so main knows to wait for this to finish
	c.addCount(c.limitCountChan, func(finished func()) {
		skipped := c.waitForLimiter(func() { // block this thread until free
			go func() {
				defer finished()
				c.addError(runner.Run(c))
				c.releaseLimiter()
			}()
		})
		if skipped {
			finished()
		}
	})
}

// Background start new go routine that will get stopped when all `Go` created ones finish
// if the bgRunner returns error it will gracefully shutdown everything else
func (c *Controller) Background(bgRunner Runner) {
	c.addCount(c.backCountChan, func(finished func()) {
		go func() {
			defer finished()
			err := bgRunner.Run(c)
			if err != nil {
				c.errorChan <- err
				c.Shutdown()
			}
		}()
	})
}

func (c *Controller) addError(err error) {
	if err == nil || err == ErrShuttingDown {
		return
	}
	select {
	case c.errorChan <- err:
		return
	default:
		return
	}
}
