package logkit

import "fmt"

// Logger is a third-party logger, whose bodies the module's load does not hold.
type Logger struct{}

func (*Logger) Infof(format string, args ...any) { fmt.Printf(format, args...) }

func (*Logger) Info(args ...any) { fmt.Println(args...) }
