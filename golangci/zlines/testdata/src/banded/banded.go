package banded

// Each want comment sits alone: two side by side would be a paragraph to fill.

// A paragraph whose breaks land inside the band draws nothing.

// the band tolerates a break placed anywhere inside it, so editing one word
// did not cascade a rewrap through the paragraph.
type tolerated struct{}

// want +2 `comment-wrap: .* short of the 70-column minimum`

// this line stops far short of the minimum
// and the words below it would happily join
type gathered struct{}

// want +2 `comment-wrap: .* over 1 lines; .* takes 2`

// this single line of prose runs well past the eighty column limit the band still enforces on every line
type overlong struct{}

// want +2 `comment-wrap: .* breaks after a word that belongs.*`

// this line reaches comfortably past column seventy and yet strands the
// reader on it.
type stranded struct{}

// A break the words force is not a break the band polices.

// this line stopped early only because the next word measures
// electroencephalography wide.
type forced struct{}

// the fill itself may close a line short when it hands the tail
// of the paragraph downward.
type demoted struct{}
