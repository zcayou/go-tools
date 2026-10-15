// leader.Context shares a name with a Ginkgo container and registers nothing,
// so this file carries no specs.
package lookalikes_test

type leader struct{}

func (leader) Context(name string) string { return "leading " + name }

func activeLeadership(l leader) string { return l.Context("widget") }
