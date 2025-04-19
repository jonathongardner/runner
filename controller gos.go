package runner

func (c *Controller) checkDone(function func()) {
	select {
	case <-c.doneChan:
		log.Debug("Not running job because shuting down")
	default:
		function()
	}
}

// Go run all of these until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()`
func (c *Controller) Go(runner Runner) {
	c.checkDone(func() {
		c.mainCountChan <- true
		go func() {
			c.addError(runner.Run(c))
			c.mainCountChan <- false
		}()
	})
}

// LimitedGo run all of these with a limit until none left
// if the runner returns error it will add to chan a
// and can be retrieved with `Errors()`
func (c *Controller) LimitedGo(runner Runner) {
	c.checkDone(func() {
		// Add count so we know to wait to close stuff
		c.limitCountChan <- true
		go func() {
			select {
			case <-c.doneChan:
				log.Debug("Not running limited job because shuting down")
			case c.limiter <- true:
				log.Info("Got it")
				c.addError(runner.Run(c))
				select {
				case <-c.limiter:
				default:
					log.Errorf("no more limiter available")
				}
				c.limitCountChan <- false
			}
		}()
	})
}

// BlLimitedGo is the same as LimitedGo but it will block adding
// to the limiter until one is free
func (c *Controller) BlLimitedGo(runner Runner) {
	c.checkDone(func() {
		// Add count so we know to wait to close stuff
		c.limitCountChan <- true
		select {
		case <-c.doneChan:
			log.Debug("Not running limited job because shuting down")
		case c.limiter <- true:
			go func() {
				log.Info("Got it")
				c.addError(runner.Run(c))
				select {
				case <-c.limiter:
				default:
					log.Errorf("no more limiter available")
				}
				c.limitCountChan <- false
			}()
		}
	})
}

// GoBackground start new go routine that will get stopped when all `Go` created ones finish
// if the bgRunner returns error it will gracefully shutdown everything else
func (c *Controller) GoBackground(bgRunner Runner) {
	c.checkDone(func() {
		c.backCountChan <- true
		go func() {
			err := bgRunner.Run(c)
			if err != nil {
				c.errorChan <- err
				c.Shutdown()
			}
			c.backCountChan <- false
		}()
	})
}

func (c *Controller) addError(err error) {
	if err == nil {
		return
	}
	select {
	case <-c.doneChan:
		return
	default:
		c.errorChan <- err
	}
}
