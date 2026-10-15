package golangci

import (
	"github.com/golangci/plugin-module-register/register"

	"github.com/zcayou/go-tools/golangci/deadcode"
	"github.com/zcayou/go-tools/golangci/zenum"
	"github.com/zcayou/go-tools/golangci/zerrors"
	"github.com/zcayou/go-tools/golangci/zlines"
	"github.com/zcayou/go-tools/golangci/ztest"
)

func init() {
	register.Plugin(ztest.Name, ztest.New)
	register.Plugin(deadcode.Name, deadcode.New)
	register.Plugin(zlines.Name, zlines.New)
	register.Plugin(zerrors.Name, zerrors.New)
	register.Plugin(zenum.Name, zenum.New)
}
