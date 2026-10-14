package enum

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
)

// Tokens are what an enum's members render as.
//
// A string enum's tokens are its members' values. An ordinal enum's are what
// its [Name] method returns for each member's value: the method is read rather
// than run, by following its body for that value. The reader follows
// assignments, if and switch statements, conversions, arithmetic and len
// of a table, and reads names out of package-level array, slice and map
// literals, which covers the hand-written forms — a switch on the receiver,
// an if chain, a table indexed by the receiver at an optional offset behind
// a bounds check — and the three layouts stringer generates.
//
// A member gets a token only where the body reaches a constant string
// for that member's value specifically: one read out of a table by an index
// computed from the receiver, or one reached inside a branch entered
// by an equality on the receiver that holds for the value — a switch case,
// a case of a tagless switch or an if condition such as k == A. A string
// the body returns before any condition, switch or table lookup is what every
// value renders as, and is each member's token. Whatever the body reaches any
// other way is its fallback for a value no member holds, and gives no token:
// a switch's default arm, the return after an if chain or a switch none
// of whose cases matched, the return after a map lookup that found nothing,
// the empty string a map holds for a key it lacks, and a panic.
//
// A String is in no readable form, and its tokens unreadable, when the body
// leaves what the reader follows for some member — a loop, a call computing
// the name, a condition on something no form names — or gives no member
// a token at all.
type Tokens struct {
	// Method is the enum's String() string method, or nil for an ordinal
	// enum that declares none.
	Method *types.Func

	// Readable reports whether the members' tokens could be read. It is always
	// true for a string enum, and false for an ordinal one with no Method.
	Readable bool

	byValue map[string]string
}

// Token returns the token a member value renders as, or false when it renders
// as none: when it reaches the fallback, or when the tokens are unreadable.
func (t Tokens) Token(value constant.Value) (string, bool) {
	text, ok := t.byValue[value.ExactString()]
	return text, ok
}

// Tokens reads the enum's tokens, an ordinal enum's from the syntax
// of the package declaring it.
func (v *Enum) Tokens() Tokens {
	tokens := Tokens{Method: Name(v.Type), byValue: map[string]string{}}
	if v.Textual() {
		tokens.Readable = true
		for _, value := range v.Values() {
			tokens.byValue[value.ExactString()] = constant.StringVal(value)
		}
		return tokens
	}
	if tokens.Method == nil || v.pkg.Info == nil {
		return Tokens{Method: tokens.Method}
	}
	decl := funcDecl(v.pkg.Files, v.pkg.Info, tokens.Method)
	if decl == nil || decl.Body == nil {
		return Tokens{Method: tokens.Method}
	}

	reader := newReader(v.pkg.Files, v.pkg.Info, decl)
	for _, value := range v.Values() {
		stop, ok := reader.read(value)
		if !ok {
			return Tokens{Method: tokens.Method}
		}
		if stop.token {
			tokens.byValue[value.ExactString()] = stop.text
		}
	}
	if len(tokens.byValue) == 0 {
		return Tokens{Method: tokens.Method}
	}
	tokens.Readable = true
	return tokens
}

func funcDecl(files []*ast.File, info *types.Info, method *types.Func) *ast.FuncDecl {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && info.Defs[fn.Name] == method {
				return fn
			}
		}
	}
	return nil
}

// reader follows one String method's body for a given receiver value.
type reader struct {
	info *types.Info
	decl *ast.FuncDecl
	recv types.Object

	// tables are the package-level variables initialized with a composite
	// literal, which a name may be read out of.
	tables map[types.Object]*ast.CompositeLit
}

func newReader(files []*ast.File, info *types.Info, decl *ast.FuncDecl) *reader {
	r := &reader{info: info, decl: decl, tables: map[types.Object]*ast.CompositeLit{}}
	if names := decl.Recv.List[0].Names; len(names) == 1 {
		r.recv = info.Defs[names[0]]
	}
	for _, file := range files {
		for _, d := range file.Decls {
			gen, ok := d.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				spec := spec.(*ast.ValueSpec)
				if len(spec.Values) != len(spec.Names) {
					continue
				}
				for i, name := range spec.Names {
					if lit, ok := ast.Unparen(spec.Values[i]).(*ast.CompositeLit); ok {
						r.tables[info.Defs[name]] = lit
					}
				}
			}
		}
	}
	return r
}

// stop is where following the body for one value ended: at a return statement,
// or at a panic.
type stop struct {
	// stopped reports that the walk reached a return or a panic.
	stopped bool
	// token reports that what was returned is the value's token.
	token bool
	text  string
}

// value is what an expression evaluates to for the receiver value being
// followed.
type value struct {
	// c is the expression's constant value, or nil when it has none.
	c constant.Value
	// table is the composite literal a package-level table holds.
	table *ast.CompositeLit
	// derived reports that the value is computed from the receiver.
	derived bool
	// pinned reports, of a string, that it was reached for this receiver value
	// specifically, and of a condition, that it holds by an equality
	// on the receiver.
	pinned bool
	// panics reports that evaluating the expression panics.
	panics bool
}

// frame is one walk of the body: the locals assigned so far, whether the walk
// is inside a branch an equality on the receiver entered, and whether it has
// passed anything that tells values apart.
type frame struct {
	locals  map[types.Object]value
	pinned  bool
	decided bool
}

// read follows the body for one receiver value. It reports false when the body
// leaves the readable forms on the way.
func (r *reader) read(recv constant.Value) (stop, bool) {
	f := &frame{locals: map[types.Object]value{}}
	if r.recv != nil {
		f.locals[r.recv] = value{c: recv, derived: true}
	}
	out, ok := r.block(f, r.decl.Body.List)
	if !ok || !out.stopped {
		return stop{}, false
	}
	return out, true
}

// halted is where a walk stops at a panic, or at a return of a fallback:
// readable, and no token.
var halted = stop{stopped: true}

func (r *reader) block(f *frame, stmts []ast.Stmt) (stop, bool) {
	for _, stmt := range stmts {
		out, ok := r.stmt(f, stmt)
		if !ok || out.stopped {
			return out, ok
		}
	}
	return stop{}, true
}

func (r *reader) stmt(f *frame, stmt ast.Stmt) (stop, bool) {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return r.block(f, stmt.List)
	case *ast.ReturnStmt:
		return r.returnStmt(f, stmt)
	case *ast.AssignStmt:
		return r.assign(f, stmt)
	case *ast.IncDecStmt:
		return r.incDec(f, stmt)
	case *ast.DeclStmt:
		return r.declare(f, stmt)
	case *ast.ExprStmt:
		if r.panics(stmt.X) {
			return halted, true
		}
		// Any other call's result is discarded, and no form reads its effects.
		return stop{}, true
	case *ast.IfStmt:
		return r.ifStmt(f, stmt)
	case *ast.SwitchStmt:
		return r.switchStmt(f, stmt)
	case *ast.EmptyStmt:
		return stop{}, true
	default:
		return stop{}, false
	}
}

// panics reports whether expr is a call to the panic builtin.
func (r *reader) panics(expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := r.info.Uses[id].(*types.Builtin)
	return ok && builtin.Name() == "panic"
}

// returnStmt ends the walk. A constant string reached for this value
// specifically is its token, and so is one reached before anything told values
// apart, which every value renders as. Anything else reached that way is in no
// readable form, and anything reached otherwise is the fallback.
func (r *reader) returnStmt(f *frame, stmt *ast.ReturnStmt) (stop, bool) {
	if len(stmt.Results) != 1 {
		return stop{}, false
	}
	v, ok := r.expr(f, stmt.Results[0])
	switch {
	case !ok:
		return stop{}, false
	case v.panics:
		return halted, true
	case f.decided && !f.pinned && !v.pinned:
		return halted, true
	case v.c != nil && v.c.Kind() == constant.String:
		return stop{stopped: true, token: true, text: constant.StringVal(v.c)}, true
	default:
		return stop{}, false
	}
}

func (r *reader) ifStmt(f *frame, stmt *ast.IfStmt) (stop, bool) {
	if stmt.Init != nil {
		if out, ok := r.stmt(f, stmt.Init); !ok || out.stopped {
			return out, ok
		}
	}
	cond, ok := r.expr(f, stmt.Cond)
	switch {
	case !ok:
		return stop{}, false
	case cond.panics:
		return halted, true
	}
	f.decided = true
	holds, ok := boolOf(cond)
	switch {
	case !ok:
		return stop{}, false
	case holds:
		return r.branch(f, cond.pinned, stmt.Body.List)
	case stmt.Else != nil:
		return r.stmt(f, stmt.Else)
	default:
		return stop{}, true
	}
}

// branch follows a branch's statements, pinned to the receiver value when
// an equality on it entered the branch.
func (r *reader) branch(f *frame, pinned bool, stmts []ast.Stmt) (stop, bool) {
	saved := f.pinned
	f.pinned = f.pinned || pinned
	out, ok := r.block(f, stmts)
	f.pinned = saved
	return out, ok
}

func (r *reader) switchStmt(f *frame, stmt *ast.SwitchStmt) (stop, bool) {
	if stmt.Init != nil {
		if out, ok := r.stmt(f, stmt.Init); !ok || out.stopped {
			return out, ok
		}
	}
	var tag *value
	if stmt.Tag != nil {
		v, ok := r.expr(f, stmt.Tag)
		switch {
		case !ok || v.c == nil && !v.panics:
			return stop{}, false
		case v.panics:
			return halted, true
		}
		tag = &v
	}
	f.decided = true

	clauses := stmt.Body.List
	matched, pinned, ok := r.selectClause(f, tag, clauses)
	if !ok || matched < 0 {
		return stop{}, ok
	}
	// A clause ending in fallthrough runs on into the next one.
	for _, clause := range clauses[matched:] {
		body := clause.(*ast.CaseClause).Body
		through := false
		if n := len(body); n > 0 {
			if branch, isBranch := body[n-1].(*ast.BranchStmt); isBranch && branch.Tok == token.FALLTHROUGH {
				body, through = body[:n-1], true
			}
		}
		out, ok := r.branch(f, pinned, body)
		if !ok || out.stopped || !through {
			return out, ok
		}
	}
	return stop{}, true
}

// selectClause returns the index of the clause a switch runs, as the language
// picks it: the first case that matches, else the default, else -1 for none.
// It reports whether an equality on the receiver picked it.
func (r *reader) selectClause(f *frame, tag *value, clauses []ast.Stmt) (int, bool, bool) {
	fallback := -1
	for i, clause := range clauses {
		clause := clause.(*ast.CaseClause)
		if clause.List == nil {
			fallback = i
			continue
		}
		hit, ok := r.matches(f, tag, clause.List)
		if !ok {
			return -1, false, false
		}
		if hit.c != nil && constant.BoolVal(hit.c) {
			return i, hit.pinned, true
		}
	}
	return fallback, false, true
}

// matches reports whether one of a case's expressions selects the case:
// equals the tag, or holds in a tagless switch. The result is pinned when
// the case is entered by an equality on the receiver.
func (r *reader) matches(f *frame, tag *value, cases []ast.Expr) (value, bool) {
	for _, expr := range cases {
		v, ok := r.expr(f, expr)
		if !ok || v.c == nil || v.panics {
			return value{}, false
		}
		if tag == nil {
			if v.c.Kind() != constant.Bool {
				return value{}, false
			}
			if constant.BoolVal(v.c) {
				return v, true
			}
			continue
		}
		if !sameKind(tag.c, v.c) {
			return value{}, false
		}
		if constant.Compare(tag.c, token.EQL, v.c) {
			return value{c: constant.MakeBool(true), pinned: tag.derived || v.derived}, true
		}
	}
	return value{c: constant.MakeBool(false)}, true
}

func (r *reader) assign(f *frame, stmt *ast.AssignStmt) (stop, bool) {
	if len(stmt.Lhs) == 2 && len(stmt.Rhs) == 1 {
		// v, ok := table[i], the map form stringer generates.
		index, ok := ast.Unparen(stmt.Rhs[0]).(*ast.IndexExpr)
		if !ok {
			return stop{}, false
		}
		elem, found, ok := r.index(f, index)
		if !ok {
			return stop{}, false
		}
		if elem.panics {
			return halted, true
		}
		return stop{}, r.bind(f, stmt.Lhs[0], elem) && r.bind(f, stmt.Lhs[1], value{c: constant.MakeBool(found)})
	}
	if len(stmt.Lhs) != len(stmt.Rhs) {
		return stop{}, false
	}
	values := make([]value, len(stmt.Rhs))
	for i, rhs := range stmt.Rhs {
		v, ok := r.expr(f, rhs)
		if !ok {
			return stop{}, false
		}
		if v.panics {
			return halted, true
		}
		if op, compound := compoundOps[stmt.Tok]; compound {
			lhs, ok := r.expr(f, stmt.Lhs[i])
			if !ok {
				return stop{}, false
			}
			if v, ok = r.operate(op, lhs, v, stmt.Lhs[i]); !ok {
				return stop{}, false
			}
		}
		values[i] = v
	}
	for i, lhs := range stmt.Lhs {
		if !r.bind(f, lhs, values[i]) {
			return stop{}, false
		}
	}
	return stop{}, true
}

var compoundOps = map[token.Token]token.Token{
	token.ADD_ASSIGN:     token.ADD,
	token.SUB_ASSIGN:     token.SUB,
	token.MUL_ASSIGN:     token.MUL,
	token.QUO_ASSIGN:     token.QUO,
	token.REM_ASSIGN:     token.REM,
	token.AND_ASSIGN:     token.AND,
	token.OR_ASSIGN:      token.OR,
	token.XOR_ASSIGN:     token.XOR,
	token.SHL_ASSIGN:     token.SHL,
	token.SHR_ASSIGN:     token.SHR,
	token.AND_NOT_ASSIGN: token.AND_NOT,
}

// incDec follows x++ and x--.
func (r *reader) incDec(f *frame, stmt *ast.IncDecStmt) (stop, bool) {
	x, ok := r.expr(f, stmt.X)
	if !ok {
		return stop{}, false
	}
	op := token.ADD
	if stmt.Tok == token.DEC {
		op = token.SUB
	}
	v, ok := r.operate(op, x, value{c: constant.MakeInt64(1)}, stmt.X)
	return stop{}, ok && r.bind(f, stmt.X, v)
}

// declare follows a local declaration. A constant's value is folded wherever
// it is used, and a variable starts at its initializer or its zero value.
func (r *reader) declare(f *frame, stmt *ast.DeclStmt) (stop, bool) {
	gen, ok := stmt.Decl.(*ast.GenDecl)
	if !ok {
		return stop{}, false
	}
	if gen.Tok != token.VAR {
		return stop{}, true
	}
	for _, spec := range gen.Specs {
		spec := spec.(*ast.ValueSpec)
		switch {
		case len(spec.Values) == 0:
			for _, name := range spec.Names {
				obj := r.info.Defs[name]
				if obj == nil || !r.bind(f, name, value{c: zero(obj.Type())}) {
					return stop{}, false
				}
			}
		case len(spec.Values) == len(spec.Names):
			for i, name := range spec.Names {
				v, ok := r.expr(f, spec.Values[i])
				if !ok {
					return stop{}, false
				}
				if v.panics {
					return halted, true
				}
				if !r.bind(f, name, v) {
					return stop{}, false
				}
			}
		default:
			return stop{}, false
		}
	}
	return stop{}, true
}

// bind assigns v to a local, the receiver included. A value bound inside
// a branch an equality on the receiver entered was reached for this value
// specifically.
func (r *reader) bind(f *frame, lhs ast.Expr, v value) bool {
	id, ok := ast.Unparen(lhs).(*ast.Ident)
	if !ok {
		return false
	}
	if id.Name == "_" {
		return true
	}
	obj := r.info.ObjectOf(id)
	if obj == nil || obj.Parent() == obj.Pkg().Scope() {
		return false
	}
	v.pinned = v.pinned || f.pinned
	f.locals[obj] = v
	return true
}

func (r *reader) expr(f *frame, expr ast.Expr) (value, bool) {
	expr = ast.Unparen(expr)
	if tv, ok := r.info.Types[expr]; ok && tv.Value != nil {
		return value{c: tv.Value}, true
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		obj := r.info.Uses[expr]
		if v, ok := f.locals[obj]; ok {
			return v, true
		}
		if table, ok := r.tables[obj]; ok {
			return value{table: table}, true
		}
		return value{}, true
	case *ast.BinaryExpr:
		return r.binary(f, expr)
	case *ast.UnaryExpr:
		return r.unary(f, expr)
	case *ast.CallExpr:
		return r.call(f, expr)
	case *ast.IndexExpr:
		elem, _, ok := r.index(f, expr)
		return elem, ok
	case *ast.SliceExpr:
		return r.slice(f, expr)
	default:
		// Anything else is a value no readable form names, which is fine
		// on a fallback path and unreadable on a member's own: the return decides.
		return value{}, true
	}
}

func (r *reader) unary(f *frame, expr *ast.UnaryExpr) (value, bool) {
	x, ok := r.expr(f, expr.X)
	if !ok || x.panics {
		return x, ok
	}
	if x.c == nil {
		return value{derived: x.derived}, true
	}
	if expr.Op == token.NOT {
		if x.c.Kind() != constant.Bool {
			return value{}, false
		}
		return value{c: constant.MakeBool(!constant.BoolVal(x.c)), derived: x.derived}, true
	}
	if x.c.Kind() == constant.Bool {
		return value{}, false
	}
	return value{c: r.fit(constant.UnaryOp(expr.Op, x.c, 0), expr), derived: x.derived, pinned: x.pinned}, true
}

func (r *reader) binary(f *frame, expr *ast.BinaryExpr) (value, bool) {
	x, ok := r.expr(f, expr.X)
	if !ok || x.panics {
		return x, ok
	}
	if expr.Op == token.LAND || expr.Op == token.LOR {
		return r.logical(f, expr, x)
	}
	y, ok := r.expr(f, expr.Y)
	if !ok || y.panics {
		return y, ok
	}
	return r.operate(expr.Op, x, y, expr)
}

// operate applies a comparison, shift or arithmetic operator. typed
// is the expression whose type an integer result wraps to. An equality
// on a value computed from the receiver that holds pins the branch it enters.
func (r *reader) operate(op token.Token, x, y value, typed ast.Expr) (value, bool) {
	derived := x.derived || y.derived
	if x.c == nil || y.c == nil {
		// A comparison on a value no readable form names cannot be followed.
		return value{derived: derived, pinned: x.pinned || y.pinned}, !comparison(op)
	}
	switch {
	case op == token.SHL || op == token.SHR:
		shift, exact := constant.Uint64Val(constant.ToInt(y.c))
		if !exact || x.c.Kind() != constant.Int {
			return value{}, false
		}
		return value{c: r.fit(constant.Shift(x.c, op, uint(shift)), typed), derived: derived}, true
	case !sameKind(x.c, y.c):
		return value{}, false
	case comparison(op):
		holds := constant.Compare(x.c, op, y.c)
		return value{c: constant.MakeBool(holds), derived: derived, pinned: op == token.EQL && holds && derived}, true
	default:
		return r.arithmetic(op, x, y, typed)
	}
}

// arithmetic applies an arithmetic operator to two constants of one kind.
// Dividing by zero panics, as running the code would.
func (r *reader) arithmetic(op token.Token, x, y value, typed ast.Expr) (value, bool) {
	if x.c.Kind() == constant.Bool {
		return value{}, false
	}
	if op == token.QUO && x.c.Kind() == constant.Int && y.c.Kind() == constant.Int {
		op = token.QUO_ASSIGN // integer division, as constant.BinaryOp spells it
	}
	if (op == token.QUO_ASSIGN || op == token.REM) && constant.Sign(y.c) == 0 {
		return value{panics: true}, true
	}
	c := r.fit(constant.BinaryOp(x.c, op, y.c), typed)
	return value{c: c, derived: x.derived || y.derived, pinned: x.pinned || y.pinned}, true
}

// logical follows && and ||, evaluating the right operand only when the left
// does not settle the result, as running the code would. The result pins
// when an operand that decided it does.
func (r *reader) logical(f *frame, expr *ast.BinaryExpr, x value) (value, bool) {
	left, known := boolOf(x)
	if !known {
		return value{}, false
	}
	if expr.Op == token.LAND && !left || expr.Op == token.LOR && left {
		return value{c: constant.MakeBool(left), pinned: left && x.pinned}, true
	}
	y, ok := r.expr(f, expr.Y)
	if !ok || y.panics {
		return y, ok
	}
	right, known := boolOf(y)
	if !known {
		return value{}, false
	}
	pinned := right && (y.pinned || expr.Op == token.LAND && x.pinned)
	return value{c: constant.MakeBool(right), pinned: pinned}, true
}

func boolOf(v value) (bool, bool) {
	if v.c == nil || v.c.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(v.c), true
}

func comparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

// call follows a conversion and len of a table. Any other call is a value
// no readable form names.
func (r *reader) call(f *frame, call *ast.CallExpr) (value, bool) {
	if tv, ok := r.info.Types[call.Fun]; ok && tv.IsType() && len(call.Args) == 1 {
		arg, ok := r.expr(f, call.Args[0])
		if !ok || arg.panics || arg.c == nil {
			return arg, ok
		}
		if basic, ok := tv.Type.Underlying().(*types.Basic); ok && basic.Info()&types.IsString != 0 &&
			arg.c.Kind() != constant.String {
			return value{derived: arg.derived}, true
		}
		arg.c = r.fit(arg.c, call)
		return arg, true
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return value{}, true
	}
	if builtin, isBuiltin := r.info.Uses[id].(*types.Builtin); !isBuiltin || builtin.Name() != "len" {
		return value{}, true
	}
	arg, ok := r.expr(f, call.Args[0])
	if !ok || arg.table == nil {
		return value{}, ok
	}
	n, ok := r.length(arg.table)
	if !ok {
		return value{}, false
	}
	return value{c: constant.MakeInt64(n)}, true
}

// length is the number of elements a table literal holds: a map's entries,
// or the length of an array or slice.
func (r *reader) length(lit *ast.CompositeLit) (int64, bool) {
	switch t := r.info.TypeOf(lit).Underlying().(type) {
	case *types.Map:
		return int64(len(lit.Elts)), true
	case *types.Array:
		return t.Len(), true
	}
	var next, size int64
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key := r.info.Types[kv.Key].Value
			if key == nil {
				return 0, false
			}
			at, exact := constant.Int64Val(constant.ToInt(key))
			if !exact {
				return 0, false
			}
			next = at
		}
		next++
		size = max(size, next)
	}
	return size, true
}

// index looks an element up in a package-level table literal. It reports
// whether the table holds an entry for the index, and false for its last result
// when the expression is in no readable form. A map gives its element's zero
// value for a key it lacks, and an array or slice panics for an index outside
// it. An element read by an index computed from the receiver is reached
// for the receiver's value specifically.
func (r *reader) index(f *frame, expr *ast.IndexExpr) (value, bool, bool) {
	x, ok := r.expr(f, expr.X)
	if !ok || x.panics {
		return x, false, ok
	}
	key, ok := r.expr(f, expr.Index)
	if !ok || key.panics {
		return key, false, ok
	}
	if x.table == nil || key.c == nil {
		return value{derived: key.derived, pinned: key.derived}, false, true
	}
	if key.derived {
		// Whether the table holds an entry tells values apart.
		f.decided = true
	}

	elem, isMap, ok := tableElem(r.info.TypeOf(x.table))
	if !ok {
		return value{}, false, false
	}
	entry, found, ok := r.entry(x.table, isMap, key.c)
	switch {
	case !ok:
		return value{}, false, false
	case found:
		v, read := r.expr(f, entry)
		v.derived = v.derived || key.derived
		v.pinned = v.pinned || key.derived
		return v, read, read
	case isMap:
		return value{c: zero(elem)}, false, true
	}
	size, ok := r.length(x.table)
	if !ok {
		return value{}, false, false
	}
	at, exact := constant.Int64Val(constant.ToInt(key.c))
	if !exact || at < 0 || at >= size {
		return value{panics: true}, false, true
	}
	return value{c: zero(elem), derived: key.derived, pinned: key.derived}, false, true
}

// tableElem returns the element type of a table's map, array or slice type,
// and whether it is a map.
func tableElem(t types.Type) (types.Type, bool, bool) {
	switch t := t.Underlying().(type) {
	case *types.Map:
		return t.Elem(), true, true
	case *types.Array:
		return t.Elem(), false, true
	case *types.Slice:
		return t.Elem(), false, true
	default:
		return nil, false, false
	}
}

// entry finds the element a table literal writes for key. It reports whether
// the literal writes one, and false for its last result when a key is no
// constant.
func (r *reader) entry(table *ast.CompositeLit, isMap bool, key constant.Value) (ast.Expr, bool, bool) {
	var next int64
	for _, elt := range table.Elts {
		entry := elt
		var at constant.Value
		if kv, keyed := elt.(*ast.KeyValueExpr); keyed {
			entry = kv.Value
			if at = r.info.Types[kv.Key].Value; at == nil {
				return nil, false, false
			}
			if !isMap {
				next, _ = constant.Int64Val(constant.ToInt(at))
			}
		} else if isMap {
			return nil, false, false
		}
		if !isMap {
			at = constant.MakeInt64(next)
			next++
		}
		if sameKind(at, key) && constant.Compare(at, token.EQL, key) {
			return entry, true, true
		}
	}
	return nil, false, true
}

func (r *reader) slice(f *frame, expr *ast.SliceExpr) (value, bool) {
	x, ok := r.expr(f, expr.X)
	if !ok || x.panics {
		return x, ok
	}
	if x.c == nil || x.c.Kind() != constant.String || expr.Slice3 {
		return value{derived: x.derived, pinned: x.pinned}, true
	}
	text := constant.StringVal(x.c)
	low, high := int64(0), int64(len(text))
	pinned, derived := x.pinned, x.derived
	for _, bound := range []struct {
		expr *ast.Expr
		into *int64
	}{{&expr.Low, &low}, {&expr.High, &high}} {
		if *bound.expr == nil {
			continue
		}
		v, ok := r.expr(f, *bound.expr)
		if !ok || v.panics {
			return v, ok
		}
		if v.c == nil {
			return value{derived: derived || v.derived, pinned: pinned || v.pinned}, true
		}
		n, exact := constant.Int64Val(constant.ToInt(v.c))
		if !exact {
			return value{}, false
		}
		*bound.into = n
		pinned = pinned || v.pinned
		derived = derived || v.derived
	}
	if low < 0 || high < low || high > int64(len(text)) {
		return value{panics: true}, true
	}
	return value{c: constant.MakeString(text[low:high]), derived: derived, pinned: pinned}, true
}

// zero is the zero value of a basic type as a constant, or nil for any other
// type.
func zero(t types.Type) constant.Value {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return nil
	}
	switch info := basic.Info(); {
	case info&types.IsString != 0:
		return constant.MakeString("")
	case info&types.IsInteger != 0:
		return constant.MakeInt64(0)
	case info&types.IsFloat != 0:
		return constant.MakeFloat64(0)
	case info&types.IsBoolean != 0:
		return constant.MakeBool(false)
	default:
		return nil
	}
}

// fit wraps an integer result to the width of the type expr has, as running
// the code would. stringer's offset subtracts from the receiver, which may
// be unsigned, before its bounds check runs.
func (r *reader) fit(v constant.Value, expr ast.Expr) constant.Value {
	if v.Kind() != constant.Int {
		return v
	}
	basic, ok := r.info.TypeOf(expr).Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 || basic.Info()&types.IsUntyped != 0 {
		return v
	}
	n, ok := new(big.Int).SetString(v.ExactString(), 10)
	if !ok {
		return v
	}
	modulus := new(big.Int).Exp(big.NewInt(2), big.NewInt(sizes.Sizeof(basic)*8), nil)
	n.Mod(n, modulus)
	if basic.Info()&types.IsUnsigned == 0 && n.Cmp(new(big.Int).Rsh(modulus, 1)) >= 0 {
		n.Sub(n, modulus)
	}
	return constant.Make(n)
}

// sizes fixes int and uint at 64 bits, which is what every platform an enum
// is realistically read on gives them.
var sizes = types.SizesFor("gc", "amd64")
