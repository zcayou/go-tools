package testifytext_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"testifytext"
)

func TestMessageArguments(t *testing.T) {
	err := testifytext.Do()

	assert.Equal(t, testifytext.ErrBoom, err, err.Error())
	assert.True(t, err != nil, err.Error())
	assert.Equal(t, testifytext.ErrBoom, err, "failed: %s", err.Error())
	assert.Equalf(t, "boom", err.Error(), "doing %s", "it") // want `^text-assertion`

	msg := err.Error()
	assert.Equal(t, "boom", msg) // want `^text-assertion`
}

func TestShadowedPackage(t *testing.T) {
	assert := assert.New(t)
	err := fmt.Errorf("wrapping: %w", testifytext.Do())

	assert.EqualError(errors.Unwrap(err), "boom")
	assert.Equal("boom", err.Error()) // want `^text-assertion`
}
