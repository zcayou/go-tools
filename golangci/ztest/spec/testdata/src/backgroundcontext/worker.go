package backgroundcontext

import "context"

// Run is production code, which no rule here judges.
func Run() context.Context { return context.Background() }
