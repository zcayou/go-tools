package engine

import (
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// maxCompletionPasses bounds how many times completion may revisit the program.
// Each pass builds what the previous one's bodies made reachable, so the count
// tracks how deep lazily built generic code nests, which no real program takes
// many levels of.
const maxCompletionPasses = 64

// program is the built SSA program every evaluation reads, together
// with the functions it holds.
//
// x/tools builds a method of an instantiated type, and the wrappers around one,
// when something first asks for it, and Rapid Type Analysis asks as it runs.
// A program read before reachability holds fewer functions than one read after,
// and evidence in a body built in between — a conversion inside a method
// of a generic type instantiated with a type a test supplies, say — would count
// in whichever evaluation came second. funcs is fixed before the first evidence
// sweep instead, so both views and every sweep within one read the same
// functions, and the two evaluations differ in exactly the one variable masking
// sets.
type program struct {
	*ssa.Program
	funcs map[*ssa.Function]bool
}

// buildProgram builds the SSA program for the loaded packages and completes it.
func buildProgram(pkgs []*packages.Package) (*program, []*ssa.Package, error) {
	prog, ssaPkgs := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	complete, err := completeProgram(prog)
	if err != nil {
		return nil, nil, err
	}
	return complete, ssaPkgs, nil
}

// completeProgram visits the program until a visit builds nothing new. Visiting
// a runtime type's methods builds them, and their bodies can make more types
// runtime types, so the function set grows over several visits before
// it settles.
//
// Settling is judged by name, not by identity. Identical instantiated types
// reach x/tools as distinct values, and a visit reaches the methods of one
// of them, chosen arbitrarily, so successive visits return different copies
// of the same wrapper or instance and the set never stops changing by identity.
// Every copy is kept: each carries the same evidence, which counts once
// however many copies say it.
func completeProgram(prog *ssa.Program) (*program, error) {
	funcs := map[*ssa.Function]bool{}
	names := map[string]bool{}
	for range maxCompletionPasses {
		grown := false
		for fn := range ssautil.AllFunctions(prog) {
			funcs[fn] = true
			if name := fn.String() + "\x00" + fn.Synthetic; !names[name] {
				names[name] = true
				grown = true
			}
		}
		if !grown {
			return &program{Program: prog, funcs: funcs}, nil
		}
	}
	return nil, fmt.Errorf("building ssa: the program's functions did not settle within %d passes", maxCompletionPasses)
}
