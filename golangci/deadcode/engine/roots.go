package engine

import (
	"fmt"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// rootsDir is the reserved in-module directory synthesized root programs live
// under. It never exists on disk — each declared file is overlaid at its own
// path inside it, one directory per file so colliding siblings coexist —
// and Analyze refuses to run when it does, rather than mixing real files into
// a synthesized package.
const rootsDir = ".deadcode-roots"

// rootProgram is one declared entry-point file and the synthesized package
// standing in for it.
type rootProgram struct {
	// source is the on-disk path relative to the load directory, the name errors
	// carry.
	source string
	// overlay is the absolute path the file's content is synthesized at.
	overlay string
	// pattern is the load pattern that brings the synthesized package in.
	pattern string
}

// synthesizedPackage reports whether the import path names a synthesized root
// program rather than a package the module holds. The directory name
// is reserved, so the path element is the whole test.
func synthesizedPackage(path string) bool {
	for element := range strings.SplitSeq(path, "/") {
		if element == rootsDir {
			return true
		}
	}
	return false
}

// loadRootPrograms resolves the declared root patterns and synthesizes one
// single-file main package per matched file — the go run model. The file's
// content joins the load as an overlay, with its build-constraint lines
// neutralized so the loader opens what the default load could not.
func loadRootPrograms(dir string, patterns []string) ([]rootProgram, map[string][]byte, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving %s: %w", dir, err)
	}
	if _, err := os.Stat(filepath.Join(absDir, rootsDir)); err == nil {
		return nil, nil, fmt.Errorf("%s exists in %s: the directory is reserved for synthesized root programs",
			rootsDir, dir)
	}

	var files []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(absDir, pattern))
		if err != nil {
			return nil, nil, fmt.Errorf("resolving roots pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, nil, fmt.Errorf("roots pattern %q matched no files", pattern)
		}
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				files = append(files, match)
			}
		}
	}

	programs := make([]rootProgram, 0, len(files))
	overlay := make(map[string][]byte, len(files))
	for i, file := range files {
		source, err := filepath.Rel(absDir, file)
		if err != nil {
			source = file
		}
		src, err := os.ReadFile(file) //nolint:gosec // the path is a file the configuration declared, inside the analyzed module
		if err != nil {
			return nil, nil, fmt.Errorf("reading root program: %w", err)
		}
		patched := neutralizeConstraints(src)
		if err := checkMainClause(source, patched); err != nil {
			return nil, nil, err
		}
		gen := fmt.Sprintf("gen%d", i)
		program := rootProgram{
			source:  source,
			overlay: filepath.Join(absDir, rootsDir, gen, "main.go"),
			pattern: "./" + rootsDir + "/" + gen,
		}
		programs = append(programs, program)
		overlay[program.overlay] = patched
	}
	return programs, overlay, nil
}

// neutralizeConstraints replaces every build-constraint line with a comment
// of equal byte length, so the loader opens the file unconditionally while
// every position it reports still matches the on-disk source. Constraints
// precede the package clause with only blank lines and line comments before
// them, so the scan stops at the first line that is neither.
func neutralizeConstraints(src []byte) []byte {
	patched := slices.Clone(src)
	offset := 0
	for line := range strings.Lines(string(src)) {
		text := strings.TrimRight(line, "\r\n")
		if trimmed := strings.TrimSpace(text); trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			break
		}
		if constraint.IsGoBuild(text) || constraint.IsPlusBuild(text) {
			copy(patched[offset:], "//")
			for i := offset + 2; i < offset+len(text); i++ {
				patched[i] = ' '
			}
		}
		offset += len(line)
	}
	return patched
}

// checkMainClause rejects a declared file that is not a main program before
// the load synthesizes a package no root selection could use.
func checkMainClause(name string, src []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.PackageClauseOnly)
	if err != nil {
		return fmt.Errorf("reading root program: %w", err)
	}
	if file.Name.Name != "main" {
		return fmt.Errorf("root program %s declares package %s, not main", name, file.Name.Name)
	}
	return nil
}

// validateRootPrograms rejects a synthesized package with no func main.
// go/types does not require one — that is the linker's rule — and without
// the check the program would contribute evidence but no root, a silent
// half-effect no declared entry point should have.
func validateRootPrograms(pkgs []*packages.Package, programs []rootProgram) error {
	sources := make(map[string]string, len(programs))
	for _, program := range programs {
		sources[program.overlay] = program.source
	}
	for _, pkg := range pkgs {
		if pkg.Types == nil || !synthesizedPackage(pkg.PkgPath) {
			continue
		}
		if _, ok := pkg.Types.Scope().Lookup("main").(*types.Func); ok {
			continue
		}
		name := pkg.PkgPath
		for _, file := range pkg.CompiledGoFiles {
			if source, ok := sources[file]; ok {
				name = source
			}
		}
		return fmt.Errorf("root program %s declares no func main", name)
	}
	return nil
}
