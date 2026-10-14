package testifyforms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testifyforms"
)

func TestPresence(t *testing.T) {
	var err error = testifyforms.Do()

	assert.NotNil(t, err)           // want `^unidentified-error: NotNil of an error says only that some error occurred`
	assert.NotNilf(t, err, "doing") // want `^unidentified-error: NotNilf of an error says only`
	require.NotNil(t, err)          // want `^unidentified-error: NotNil`
	assert.New(t).NotNil(err)       // want `^unidentified-error: NotNil`
	assert.Nil(t, err)

	got := testifyforms.Do()
	assert.NotNil(t, got) // want `^unidentified-error: NotNil`
	value := 1
	assert.NotNil(t, &value)
}
