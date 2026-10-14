package zenum

import "github.com/zcayou/go-tools/golangci/zenum/engine"

// The specs are black-box files like every other test in the module. This
// bridge exposes what a plugin decoded its settings into, so a spec can assert
// how a setting reads rather than only that it was accepted.

// EngineConfig exposes the configuration the plugin hands the engine.
func (p *Plugin) EngineConfig() engine.Config {
	return p.config
}

// BuildTags exposes the build tags the plugin loads the module under.
func (p *Plugin) BuildTags() []string {
	return p.buildTags
}
