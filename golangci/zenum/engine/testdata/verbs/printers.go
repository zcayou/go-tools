package verbs

import (
	"fmt"

	"example.com/logkit"
)

func logf(format string, args ...any) { fmt.Printf(format, args...) }

func fail(msg string, args ...any) { panic(fmt.Sprintf(msg, args...)) }

func chain(format string, args ...any) { logf(format, args...) }

func tell(args ...any) { fmt.Println(args...) }

func halff(format string, args ...any) { _, _ = format, args }

func fixed(format string, args ...any) { fmt.Printf("%v", args...) }

func tainted(format string, args ...any) {
	args = append(args, 1)
	fmt.Printf(format, args...)
}

type Logger interface {
	Logf(format string, args ...any)
}

type stdLogger struct{}

func (stdLogger) Logf(format string, args ...any) { fmt.Printf(format, args...) }

func newLogger() Logger { return stdLogger{} }

func printers(k Kind, j job, kit *logkit.Logger) {
	logf("%d", k)
	fail("%x", k)
	chain("%d", k)
	tell(j)
	halff("%d", k)
	fixed("%d", k)
	tainted("%d", k)
	newLogger().Logf("%d", k)
	kit.Infof("%d", k)
	kit.Info(j)
}
