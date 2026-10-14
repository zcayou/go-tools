package tokenforms

import "tokenforms/states"

type Job struct {
	State states.State
	Mode  states.Mode
}
