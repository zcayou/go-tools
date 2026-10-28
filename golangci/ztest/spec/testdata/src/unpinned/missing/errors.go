package missing // want package:"pins"

type Missing struct{} // want Missing:"rendering"

func (Missing) Error() string { return "missing" } // want `^unpinned-rendering: missing\.Missing has no errors_test\.go to pin its rendering in$`

type quiet struct{}

func (quiet) Error() string { return "quiet" }

type inner struct{}

func (inner) Error() string { return "inner" }

// Promoted renders through the Error it embeds, which it does not declare.
type Promoted struct {
	inner
}
