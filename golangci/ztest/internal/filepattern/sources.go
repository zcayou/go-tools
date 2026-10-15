package filepattern

import (
	"slices"
	"strings"
)

// Sources are the names of the files in one directory that are not test files,
// which is what [SourceToken] resolves against.
type Sources []string

// Declares reports whether a source file named base sits in the directory.
// A build-constrained variant counts: platform_linux.go is the source
// platform_test.go exercises, so a split source is not read as a missing one.
func (s Sources) Declares(base string) bool {
	return slices.ContainsFunc(s, func(name string) bool { return isSource(name, base) })
}

// Files returns every file of the directory that is the source named base:
// base.go and each of its build-constrained variants.
func (s Sources) Files(base string) []string {
	var files []string
	for _, name := range s {
		if isSource(name, base) {
			files = append(files, name)
		}
	}
	return files
}

func isSource(name, base string) bool {
	return name == base+".go" || isBuildVariant(name, base)
}

// isBuildVariant reports whether name is base under a platform file-name
// suffix: a GOOS or GOARCH constraint, the implicit build tag go/build reads
// out of a name, or _unix. go/build reads no constraint out of _unix, but
// it is how the unix side of a source split along the unix build tag is named,
// and the test for that source is named for the source all the same.
func isBuildVariant(name, base string) bool {
	rest, ok := strings.CutPrefix(name, base+"_")
	if !ok {
		return false
	}
	if rest, ok = strings.CutSuffix(rest, ".go"); !ok {
		return false
	}
	if goosValues[rest] || goarchValues[rest] {
		return true
	}
	goos, goarch, ok := strings.Cut(rest, "_")
	return ok && goosValues[goos] && goarchValues[goarch]
}

// goosValues and goarchValues are the values go/build recognizes
// in the trailing element of a file name, which is where the constraint a name
// carries comes from. They are copied from the toolchain's internal/syslist,
// which nothing exported carries, and goosValues adds unix for the reason
// [isBuildVariant] gives.
var goosValues = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "nacl": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "unix": true, "wasip1": true, "windows": true,
	"zos": true,
}

var goarchValues = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true,
	"arm64be": true, "loong64": true, "mips": true, "mipsle": true, "mips64": true, "mips64le": true,
	"mips64p32": true, "mips64p32le": true, "ppc": true, "ppc64": true, "ppc64le": true,
	"riscv": true, "riscv64": true, "s390": true, "s390x": true, "sparc": true, "sparc64": true,
	"wasm": true,
}
