// Package assert stands in for testify's assert, which is not on a fixture's
// import path since analysistest loads with GOPROXY=off. It declares the part
// of the package the fixtures call.
package assert

type TestingT interface {
	Errorf(format string, args ...any)
}

func Error(t TestingT, err error, msgAndArgs ...any) bool { return true }

func Errorf(t TestingT, err error, msg string, args ...any) bool { return true }

func NoError(t TestingT, err error, msgAndArgs ...any) bool { return true }

func ErrorIs(t TestingT, err, target error, msgAndArgs ...any) bool { return true }

func EqualError(t TestingT, theError error, errString string, msgAndArgs ...any) bool { return true }

func EqualErrorf(t TestingT, theError error, errString string, msg string, args ...any) bool {
	return true
}

func ErrorContains(t TestingT, theError error, contains string, msgAndArgs ...any) bool { return true }

func Equal(t TestingT, expected, actual any, msgAndArgs ...any) bool { return true }

func NotEqual(t TestingT, expected, actual any, msgAndArgs ...any) bool { return true }

func Contains(t TestingT, s, contains any, msgAndArgs ...any) bool { return true }

func NotContains(t TestingT, s, contains any, msgAndArgs ...any) bool { return true }

func Regexp(t TestingT, rx, str any, msgAndArgs ...any) bool { return true }

type Assertions struct {
	t TestingT
}

func New(t TestingT) *Assertions { return &Assertions{t: t} }

func (a *Assertions) Error(err error, msgAndArgs ...any) bool { return true }

func (a *Assertions) ErrorIs(err, target error, msgAndArgs ...any) bool { return true }

func (a *Assertions) EqualError(theError error, errString string, msgAndArgs ...any) bool {
	return true
}

func (a *Assertions) Equal(expected, actual any, msgAndArgs ...any) bool { return true }

func (a *Assertions) NotContains(s, contains any, msgAndArgs ...any) bool { return true }

func True(t TestingT, value bool, msgAndArgs ...any) bool { return true }

func Nil(t TestingT, object any, msgAndArgs ...any) bool { return true }

func NotNil(t TestingT, object any, msgAndArgs ...any) bool { return true }

func NotNilf(t TestingT, object any, msg string, args ...any) bool { return true }

func (a *Assertions) NotNil(object any, msgAndArgs ...any) bool { return true }

func Equalf(t TestingT, expected, actual any, msg string, args ...any) bool { return true }
