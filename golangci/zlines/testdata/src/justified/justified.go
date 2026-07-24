package justified

import "context"

// Joined this would run well past the limit, so the wrap is what keeps it
// readable.
func tooLongToJoin(
	averyveryverylongparametername int,
	anotherextremelylongparametername string,
	yetanotherlongparameternamehere []map[string]context.Context,
	andonemoreforgoodmeasureplease chan<- struct{},
) (someverylongresultnamehere int, anotherlongresultnamegoeshere error) {
	return 0, nil
}

// A line comment documenting a parameter is a reason the signature is wrapped,
// and no single-line rendering keeps it.
func documentedParams(
	count int, // how many of them
	name string,
) error {
	return nil
}

// gofmt spreads a struct type holding more than one field over several lines
// however it is written, so joining this would not outlast the formatter.
func multiFieldStruct(
	s struct {
		Count int
		Name  string
	},
) error {
	return nil
}

// A block comment given a line of its own documents the parameter beneath it.
// Joined, gofmt would sit it after the parameter above, where it would read as
// documenting that one instead.
func ownLineComment(
	count int,
	/* the name to file it under */
	name string,
) error {
	return nil
}

// A comment spanning lines cannot sit on one.
func spanningComment(
	count int, /* how
	many of them */
	name string,
) error {
	return nil
}

// Nothing to join.
func alreadyOneLine(count int, name string) error {
	return nil
}
