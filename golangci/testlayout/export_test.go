package testlayout

// The specs are black-box files like every other test in the module. This
// bridge re-exports the platform names a source split is recognized under,
// so a spec can hold them to the toolchain's own, and carries nothing else.

// GOOSValues exposes goosValues.
var GOOSValues = goosValues

// GOARCHValues exposes goarchValues.
var GOARCHValues = goarchValues
