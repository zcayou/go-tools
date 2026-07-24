package linedirective

// The directive below numbers what follows it from 2, a line this file went
// past long ago. Nothing fails on a number that low, which is the point of it:
// the rule reads a line that really is there, just not the one it was looking
// at, and drops the diagnostic or lays the fix out under the wrong indent.

type filler struct{}

type alsoFiller struct{}

//line templated.go:2

type indented struct {
	// want +2 `comment-wrap: .* over 2 lines; .* takes 1`

	// a field comment
	// wrapped short
	name string
}

// want +4 `signature-wrap: .* one 35-column line`

// want +4 `body-collapse: lowNumberedFn:`

func lowNumberedFn(
	count int,
) int { return count }
