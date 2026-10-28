package booleanidentity_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"booleanidentity"
)

func TestTestifyReported(t *testing.T) {
	err := booleanidentity.Do()

	assert.True(t, errors.Is(err, booleanidentity.ErrMissing))                      // want `^boolean-identity: asserts whether errors\.Is found its target, so a failure shows no error; use ErrorIs$`
	assert.False(t, errors.Is(err, booleanidentity.ErrClosed))                      // want `use NotErrorIs$`
	assert.Truef(t, errors.Is(err, booleanidentity.ErrMissing), "is %s", "missing") // want `use ErrorIs$`

	var target *booleanidentity.CodeError
	assert.True(t, errors.As(err, &target))  // want `^boolean-identity: asserts whether errors\.As found its target, so a failure shows no error; use ErrorAs$`
	assert.False(t, errors.As(err, &target)) // want `use NotErrorAs$`

	// The ok-check before a dereference: ErrorAs prints the chain it searched.
	coded, ok := errors.AsType[*booleanidentity.CodeError](err)
	require.True(t, ok) // want `^boolean-identity: asserts whether errors\.AsType found its target, so a failure shows no error; use ErrorAs$`
	assert.Equal(t, 1, coded.Code)

	assert.Equal(t, true, errors.Is(err, booleanidentity.ErrMissing))                        // want `use ErrorIs$`
	assert.Equal(t, errors.Is(err, booleanidentity.ErrClosed), false)                        // want `use NotErrorIs$`
	assert.EqualValues(t, true, errors.Is(err, booleanidentity.ErrMissing))                  // want `use ErrorIs$`
	assert.Exactly(t, true, errors.Is(err, booleanidentity.ErrMissing))                      // want `use ErrorIs$`
	assert.Exactlyf(t, true, errors.Is(err, booleanidentity.ErrMissing), "is %s", "missing") // want `use ErrorIs$`
	assert.NotEqual(t, true, errors.Is(err, booleanidentity.ErrClosed))                      // want `use NotErrorIs$`
	assert.NotEqualValues(t, false, errors.Is(err, booleanidentity.ErrMissing))              // want `use ErrorIs$`

	found := errors.Is(err, booleanidentity.ErrMissing)
	assert.True(t, found)                                                                 // want `use ErrorIs$`
	assert.True(t, !errors.Is(err, booleanidentity.ErrClosed))                            // want `use NotErrorIs$`
	assert.True(t, errors.Is(err, booleanidentity.ErrMissing) && errors.As(err, &target)) // want `^boolean-identity: asserts whether errors\.Is and errors\.As found their targets, so a failure shows no error; use ErrorIs or ErrorAs$`

	a := assert.New(t)
	a.True(errors.Is(err, booleanidentity.ErrMissing)) // want `use ErrorIs$`
}

func TestTestifyAdmitted(t *testing.T) {
	err := booleanidentity.Do()

	assert.ErrorIs(t, err, booleanidentity.ErrMissing)
	assert.NotErrorIs(t, err, booleanidentity.ErrClosed)
	var target *booleanidentity.CodeError
	assert.ErrorAs(t, err, &target)

	if errors.Is(err, booleanidentity.ErrMissing) {
		assert.Equal(t, "missing", "missing")
	}
	mixed := errors.Is(err, booleanidentity.ErrMissing)
	mixed = len(t.Name()) > 0
	assert.True(t, mixed)

	// The message after the values under test asserts nothing.
	assert.True(t, len(t.Name()) > 0, errors.Is(err, booleanidentity.ErrMissing))
}
