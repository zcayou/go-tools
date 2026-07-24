//line generated.go:5000

package linedirective

// A //line directive moves the numbers a position reports away from the lines
// the file actually has, and here it moves them past its end. Every rule finds
// a line by scanning the source for the break before it, so none of them reads
// a line the file does not have.

// want +2 `comment-wrap: .* over 2 lines; .* takes 1`

// a comment
// wrapped short
type shifted struct{}

// want +4 `signature-wrap: .* one 31-column line`

// want +4 `body-collapse: shiftedFn:`

func shiftedFn(
	count int,
) int { return count }
