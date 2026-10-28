// Package require stands in for testify's require, which is not on a fixture's
// import path since analysistest loads with GOPROXY=off. It declares the part
// of the package the fixtures call.
package require

type TestingT interface {
	Errorf(format string, args ...any)
	FailNow()
}

func Error(t TestingT, err error, msgAndArgs ...any) {}

func Errorf(t TestingT, err error, msg string, args ...any) {}

func ErrorContains(t TestingT, theError error, contains string, msgAndArgs ...any) {}

func Equal(t TestingT, expected, actual any, msgAndArgs ...any) {}

type Assertions struct {
	t TestingT
}

func New(t TestingT) *Assertions { return &Assertions{t: t} }

func (a *Assertions) Error(err error, msgAndArgs ...any) {}

func NotNil(t TestingT, object any, msgAndArgs ...any) {}

func True(t TestingT, value bool, msgAndArgs ...any) {}

func False(t TestingT, value bool, msgAndArgs ...any) {}

func EqualError(t TestingT, theError error, errString string, msgAndArgs ...any) {}

func Panics(t TestingT, f func(), msgAndArgs ...any) {}

func Equalf(t TestingT, expected, actual any, msg string, args ...any) {}
