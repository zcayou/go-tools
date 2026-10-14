// Package suite stands in for testify's suite, declaring the part the fixtures
// use: a Suite whose assertions are its embedded *assert.Assertions.
package suite

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Suite struct {
	*assert.Assertions

	require *require.Assertions
}

func (s *Suite) Require() *require.Assertions { return s.require }
