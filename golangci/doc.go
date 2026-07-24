// Package golangci registers every linter in this module with golangci-lint.
//
// It is the single import path a consumer names in .custom-gcl.yml, which
// builds the binary:
//
//	version: {{ the golangci-lint version to build }}
//	plugins:
//	  - module: 'github.com/zcayou/go-tools'
//	    import: 'github.com/zcayou/go-tools/golangci'
//	    version: {{ a released tag of this module }}
//
// Blank importing this package runs the registrations in register.go, after
// which each linter is addressable by name in .golangci.yml under
// linters.settings.custom.
//
// The linters themselves live in subpackages of this one. Everything under this
// directory is the golangci-lint integration and nothing else is, which is what
// keeps the rest of the module free to grow tools that have no relationship
// to it.
//
// Registration is centralized here rather than in each linter's init so
// that the linter packages stay free of import side effects and can
// be constructed directly in tests.
//
// # Adding a linter
//
// Give the linter package an exported Name constant and a New function of type
// [register.NewPlugin], then add one line to register.go. The .custom-gcl.yml
// of every consumer keeps working unchanged, because the import path does not
// move.
//
// [register.NewPlugin]: https://pkg.go.dev/github.com/golangci/plugin-module-register/register#NewPlugin
package golangci
