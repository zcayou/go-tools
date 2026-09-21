// leader.Context shares a name with a Ginkgo container and registers nothing,
// so this file carries no specs, and nor does a Describe the package declares
// for itself make it a Ginkgo package.
package lookalikes_test

type leader struct{}

func (leader) Context(name string) string { return "leading " + name }

func activeLeadership(l leader) string { return l.Context("widget") }

func Describe(text string, body func()) bool { body(); return true }
