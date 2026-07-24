package golangci

import (
	"github.com/golangci/plugin-module-register/register"

	"github.com/zcayou/go-tools/golangci/deadcode"
	"github.com/zcayou/go-tools/golangci/testlayout"
	"github.com/zcayou/go-tools/golangci/zlines"
)

func init() {
	register.Plugin(testlayout.Name, testlayout.New)
	register.Plugin(deadcode.Name, deadcode.New)
	register.Plugin(zlines.Name, zlines.New)
}
