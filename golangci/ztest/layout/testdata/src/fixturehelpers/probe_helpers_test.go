package fixturehelpers_test

type probe struct{}

func (probe) read() int { return probed() }
