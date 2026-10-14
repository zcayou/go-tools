package testifyforms_test

import (
	"testing"

	. "github.com/stretchr/testify/assert"

	"testifyforms"
)

func TestDot(t *testing.T) {
	Error(t, testifyforms.Do()) // want `^unidentified-error: Error says only`
	ErrorIs(t, testifyforms.Do(), testifyforms.ErrBoom)
}
