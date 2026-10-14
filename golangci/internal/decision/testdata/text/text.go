package text

import (
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile("x")

func Contains(err error) bool {
	return strings.Contains(err.Error(), "missing")
}

func Lowered(err error) bool {
	return strings.HasPrefix(strings.ToLower(err.Error()), "x")
}

func Sliced(err error) bool {
	return err.Error()[:3] == "abc"
}

func Local(err error, other string) bool {
	msg := other
	if err != nil {
		msg = err.Error()
	}
	return msg != "boom"
}

func Switch(err error) int {
	switch err.Error() {
	case "a":
		return 1
	case "b":
		return 2
	}
	return 0
}

func Pattern(err error) bool {
	return pattern.MatchString(err.Error())
}

func PatternFunc(err error) bool {
	ok, _ := regexp.MatchString("x", err.Error())
	return ok
}

func Escaped(err error) bool {
	msg := err.Error()
	p := &msg
	return *p == "x"
}

func Empty(err error) bool {
	return err.Error() == ""
}

func Both(a, b error) bool {
	return a.Error() == b.Error()
}

func Ordered(a, b error) int {
	return strings.Compare(a.Error(), b.Error())
}

func Unrelated(s string) bool {
	return s == "x"
}

func Parsed(err error, input string) bool {
	return strings.EqualFold(err.Error(), input)
}

func CountedEmpty(err error) int {
	return strings.Count(err.Error(), "")
}

func Rune(err error) bool {
	return strings.ContainsRune(err.Error(), ':')
}
