# Runner
A simple go package for managing multiple go routines.

## Controller
This controls all the threads. Methods to start other threads:
  - `Go`: runs until all these are finished, then `Wait` is returned
  - `LimitedGo`: same as `Go` but only runs the number of routines set by the controller limit at a time
  - `BlLimitedGo`: same as `LimitedGo` but blocks returning from the method until a routine is started.
  - `Run`: runs in line
  - `Background`: Runs in the background, should periodically check `IsShutingDown...` to finish up.