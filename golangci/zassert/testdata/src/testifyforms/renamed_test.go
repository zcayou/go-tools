package testifyforms_test

import (
	"testing"

	tassert "github.com/stretchr/testify/assert"

	"testifyforms"
)

func TestRenamed(t *testing.T) {
	tassert.Error(t, testifyforms.Do()) // want `^unidentified-error: Error says only`
}
