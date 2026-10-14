package testifyforms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"testifyforms"
)

func TestPackage(t *testing.T) {
	err := testifyforms.Do()

	assert.Error(t, err)                     // want `^unidentified-error: Error says only that some error occurred`
	assert.Errorf(t, err, "doing %s", "it")  // want `^unidentified-error: Errorf says only`
	require.Error(t, err)                    // want `^unidentified-error: Error says only`
	require.Errorf(t, err, "doing %s", "it") // want `^unidentified-error: Errorf says only`
	assert.ErrorIs(t, err, testifyforms.ErrBoom)
	assert.NoError(t, nil)
}

func TestAssertions(t *testing.T) {
	err := testifyforms.Do()

	a := assert.New(t)
	a.Error(err) // want `^unidentified-error: Error says only`
	a.ErrorIs(err, testifyforms.ErrBoom)

	r := require.New(t)
	r.Error(err) // want `^unidentified-error: Error says only`

	assert.New(t).Error(err) // want `^unidentified-error: Error says only`

	func() {
		a.Error(err) // want `^unidentified-error: Error says only`
	}()
}

// thingSuite is a testify suite, whose assertions go through the receiver.
type thingSuite struct {
	suite.Suite
}

func (s *thingSuite) TestThroughReceiver() {
	s.Error(testifyforms.Do())           // want `^unidentified-error: Error says only`
	s.Require().Error(testifyforms.Do()) // want `^unidentified-error: Error says only`
}

func elsewhere(a *assert.Assertions) {
	a.Error(testifyforms.Do()) // want `^unidentified-error: Error says only`
}

func TestElsewhere(t *testing.T) {
	elsewhere(assert.New(t))
	(&thingSuite{}).TestThroughReceiver()
}
