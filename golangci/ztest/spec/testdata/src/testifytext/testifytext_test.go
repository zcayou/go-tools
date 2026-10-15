package testifytext_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testifytext"
)

func TestText(t *testing.T) {
	err := testifytext.Do()

	assert.EqualError(t, err, "boom")                    // want `^text-assertion`
	assert.EqualErrorf(t, err, "boom", "doing %s", "it") // want `^text-assertion`
	assert.ErrorContains(t, err, "bo")                   // want `^text-assertion`
	require.ErrorContains(t, err, "bo")                  // want `^text-assertion`
	assert.Equal(t, "boom", err.Error())                 // want `^text-assertion`
	assert.Contains(t, err.Error(), "bo")                // want `^text-assertion`
	assert.Regexp(t, "^bo+m$", err.Error())              // want `^text-assertion`
	require.Equal(t, "boom", err.Error())                // want `^text-assertion`

	a := assert.New(t)
	a.EqualError(err, "boom")    // want `^text-assertion`
	a.Equal("boom", err.Error()) // want `^text-assertion`
}

func TestExempt(t *testing.T) {
	err := fmt.Errorf("wrapping: %w", testifytext.Do())

	assert.NotContains(t, err.Error(), "secret")
	assert.NotEqual(t, "secret", err.Error())
	assert.New(t).NotContains(err.Error(), "secret")

	assert.EqualError(t, errors.Unwrap(err), "boom")
	cause := errors.Unwrap(err)
	assert.Equal(t, "boom", cause.Error())
	assert.ErrorContains(t, cause, "bo")
}
