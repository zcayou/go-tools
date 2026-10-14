package golangci

import (
	"github.com/golangci/plugin-module-register/register"

	"github.com/zcayou/go-tools/golangci/deadcode"
	"github.com/zcayou/go-tools/golangci/testlayout"
	"github.com/zcayou/go-tools/golangci/zassert"
	"github.com/zcayou/go-tools/golangci/zenum"
	"github.com/zcayou/go-tools/golangci/zerrors"
	"github.com/zcayou/go-tools/golangci/zlines"
)

func init() {
	register.Plugin(testlayout.Name, testlayout.New)
	register.Plugin(deadcode.Name, deadcode.New)
	register.Plugin(zlines.Name, zlines.New)
	register.Plugin(zerrors.Name, zerrors.New)
	register.Plugin(zenum.Name, zenum.New)
	register.Plugin(zassert.Name, zassert.New)
}
