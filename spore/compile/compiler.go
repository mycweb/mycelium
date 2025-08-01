package compile

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"

	"go.brendoncarroll.net/exp/slices2"
	"go.brendoncarroll.net/stdctx/logctx"
)

type (
	Value     = myc.Value
	Expr      = mycexpr.Expr
	EB        = mycexpr.EB
	Namespace = myccanon.Namespace
	Product   = myc.Product
)

// MacroFunc takes an expression and produces an expression
type MacroFunc = func(x ast.SExpr) (ast.Node, error)

// BuiltInFunc takes an expression and produces a value.
type BuiltInFunc = func(ctx context.Context, eb EB, loc Loc, scope *Scope, args ast.SExpr) (*Expr, error)

type Compiler struct {
	s         cadata.Store
	rootScope Scope

	vm       *mvm1.VM
	macros   map[ast.Symbol]MacroFunc
	builtIns map[ast.Op]BuiltInFunc
	prims    map[ast.Op]spec.Op
}

func New(s cadata.Store, preamble map[string]*Expr) Compiler {
	c := Compiler{
		s: s,
		rootScope: Scope{
			NS: preamble,
		},
		vm: mvm1.New(0, s, mvm1.DefaultAccels()),
	}
	c.macros = map[ast.Symbol]MacroFunc{
		"b8":       makeB8,
		"b32":      makeB32,
		"distinct": makeDistinct,

		"Bit":      makeBitType,
		"Array":    makeArrayType,
		"Ref":      makeRefType,
		"Sum":      makeSumType,
		"Product":  makeProductType,
		"List":     makeListType,
		"Lazy":     makeLazyType,
		"Lambda":   makeLambdaType,
		"Fractal":  makeFractalType,
		"Distinct": makeDistinctType,
		"Port":     makePortType,
		"AnyType":  anyTypeType,
		"AnyValue": anyValueType,

		"lambda": lambdaMacro,
		"let":    letMacro,
		"if":     ifMacro,
		"eq?":    eqMacro,
		"def":    defMacro,
		"defl":   deflMacro,
		"defc":   defcMacro,
		"defm":   defmMacro,
		"pub":    pubMacro,
		"self":   selfMacro,
		"do":     doMacro,
	}
	c.builtIns = map[ast.Op]BuiltInFunc{
		"comptime": c.comptime,
		"scope":    c.scope,
		"def":      c.def,
		"do":       c.do,
		"ref":      c.ref,
		"self":     c.self,
		"kind":     c.kind,
		"macro":    c.macro,
	}
	c.prims = Primitives()
	return c
}

// Package is the output of a compilation.
// SourceFiles => | Compiler | => Package
type Package struct {
	// NS contains the exported namespace for the package
	NS Namespace
	// Internals is the internal namespace for the package
	Internals map[string]*Expr
}

// MakePackageName makes a package name from an import path
func MakePackageName(p string) ast.Symbol {
	parts := strings.Split(p, "/")
	return ast.Symbol(parts[len(parts)-1])
}

// Compile produces a myc.Namespace from a set of files.
// `base` contains precompiled objects that should be included in the package.
func (sc *Compiler) Compile(ctx context.Context, base Namespace, deps map[string]Package, files []SourceFile) (*Package, error) {
	slices.SortStableFunc(files, func(a, b SourceFile) int {
		return cmp.Compare(a.Filename, b.Filename)
	})
	for i := 1; i < len(files); i++ {
		if files[i].Filename == files[i-1].Filename {
			return nil, fmt.Errorf("duplicate source file %q", files[i].Filename)
		}
	}

	// new definitions will go to localNS as we evaluate the file contents
	localNS := make(map[string]*Expr)
	for k, v := range base {
		localNS[k] = mycexpr.Literal(v)
	}
	pkgCtx := &pkgContext{
		deps:        deps,
		localNS:     localNS,
		declaredPub: map[ast.Symbol]struct{}{},
	}
	for _, file := range files {
		if err := sc.compileFile(ctx, pkgCtx, &file); err != nil {
			return nil, err
		}
	}
	// go through everything declared pub and copy it into the pub namespace
	pubNS := map[string]Value{}
	for sym := range pkgCtx.declaredPub {
		k := string(sym)
		if _, exists := pkgCtx.localNS[k]; !exists {
			// this would be using pub on an undefined symbol
			return nil, fmt.Errorf("symbol %v is not defined by any file", sym)
		}
		expr := pkgCtx.localNS[k]
		if expr.IsLiteral() {
			pubNS[k] = expr.Value()
		} else {
			return nil, fmt.Errorf("cannot publish non-literal %v", expr)
		}
	}
	return &Package{
		NS:        pubNS,
		Internals: pkgCtx.localNS,
	}, nil
}

type pkgContext struct {
	deps map[string]Package

	localNS     map[string]*Expr
	declaredPub map[ast.Symbol]struct{}
}

func (sc *Compiler) compileFile(ctx context.Context, pkgCtx *pkgContext, file *SourceFile) error {
	importNS := Namespace{}
	for i, node := range file.Nodes {
		loc := Loc{uint32(i)}
		fctx := fileContext{
			Pkg:        pkgCtx,
			SourceFile: file,
			Loc:        loc,
			ImportNS:   importNS,
		}
		if err := sc.compileTopLevel(ctx, fctx, node); err != nil {
			if e, ok := err.(Error); ok {
				if len(e.Loc) < len(loc) {
					e.Loc = loc
				}
				if e.Source == nil {
					e.Source = file
				}
				return err
			} else {
				return Error{
					Cause:  err,
					Loc:    loc,
					Source: file,
				}
			}
		}
	}
	return nil
}

type fileContext struct {
	// Pkg is the package context that the file is from.
	Pkg *pkgContext
	// SourceFile is the SourceFile, that this expresion is from
	SourceFile *SourceFile
	// Loc a location in a source file.
	Loc Loc
	// ImportNS is the namespace
	ImportNS Namespace
}

// compileTopLevel compiles a single top level ast.Node in a file
func (sc *Compiler) compileTopLevel(ctx context.Context, fctx fileContext, x ast.Node) error {
	var eb EB
	switch x := x.(type) {
	case ast.SExpr:
		// macro
		if len(x) > 0 {
			if sym, ok := x[0].(ast.Symbol); ok {
				if macro, ok := sc.macros[sym]; ok {
					x2, err := macro(x[1:])
					if err != nil {
						return nil
					}
					return sc.compileTopLevel(ctx, fctx, x2)
				}
			}
		}
		pkgCtx := fctx.Pkg
		importNS := fctx.ImportNS
		// top-level directives
		switch {
		case isImportStatement(x):
			return sc.handleImport(importNS, pkgCtx.deps, x)
		case isDef(x):
			scope := &sc.rootScope
			scope = scope.Child(mapMap(importNS, func(k string, v Value) (string, *Expr) {
				return k, lit(v)
			}))
			scope = scope.Child(pkgCtx.localNS)
			_, err := sc.def(ctx, eb, fctx.Loc, scope, x[1:])
			return err
		case isPub(x):
			return sc.handlePub(pkgCtx.declaredPub, x[1:])
		}
	case ast.Comment:
	default:
		return fmt.Errorf("don't know what to do with %v", x)
	}
	return nil
}

// handleImport parses an import statement and modifies the fileNS accordingly
func (*Compiler) handleImport(importNS Namespace, deps map[string]Package, stmt ast.SExpr) error {
	istmt, err := asImportStmt(stmt)
	if err != nil {
		return err
	}
	pkg, found := deps[istmt.Target]
	if !found {
		return fmt.Errorf("no package provided for %q", istmt.Target)
	}
	importAs := func() string {
		if istmt.As != "" {
			return string(istmt.As)
		}
		return string(MakePackageName(istmt.Target))
	}()
	mount(importNS, importAs, pkg.NS)
	return nil
}

// handlePub handles the top-level pub directive
func (*Compiler) handlePub(pub map[ast.Symbol]struct{}, sexpr ast.SExpr) error {
	for _, expr := range sexpr {
		switch x := expr.(type) {
		case ast.Comment:
		case ast.Symbol:
			sym := x
			if _, exists := pub[sym]; exists {
				return fmt.Errorf("symbol %v declared pub more than once", sym)
			}
			pub[sym] = struct{}{}
		default:
			return fmt.Errorf("arguments to pub must be symbols have %v: %T", expr, expr)
		}
	}
	return nil
}

// CompileAST compiles a single ast.Node into an expression.
func (sc *Compiler) CompileAST(ctx context.Context, e ast.Node) (*Expr, error) {
	var eb EB
	return sc.compileAST(ctx, eb, Loc{}, sc.rootScope.Child(nil), nil, e)
}

// compileExpr compiles an ast.Node into either a mycelium Value or Expr
// Values: Int, String, Ref
func (sc *Compiler) compileAST(ctx context.Context, eb EB, loc Loc, scope *Scope, ty myc.Type, e ast.Node) (*Expr, error) {
	if e == nil {
		return nil, errors.New("a nil expr from the parser signals EOF")
	}
	switch e := e.(type) {
	case ast.Int:
		return sc.compileInt(eb, ty, e)
	case ast.String:
		return eb.String(string(e)), nil
	case ast.Ref:
		logctx.Warnf(ctx, "untyped ref")
		return eb.Lit(myc.NewRef(nil, e)), nil
	case ast.Param:
		return mycexpr.Param(uint32(e)), nil
	case ast.Array:
		return sc.compileArray(ctx, eb, loc, scope, ty, e)
	case ast.Tuple:
		return sc.compileTuple(ctx, eb, loc, scope, e)
	case ast.SExpr:
		return sc.compileSExpr(ctx, eb, loc, scope, e)
	case ast.Symbol:
		if fn, ok := sc.macros[e]; ok {
			e2, err := fn(nil)
			if err != nil {
				return nil, err
			}
			return sc.compileAST(ctx, eb, loc, scope, ty, e2)
		}
		v := scope.Get(string(e))
		if v == nil {
			return nil, fmt.Errorf("no definition for symbol %v", e)
		}
		return v, nil
	case ast.Quote:
		return lit(ASTToMycelium(e.X)), nil
	case ast.Comment:
		return lit(myc.Product{myc.NewString(";;")}), nil
	default:
		panic(e)
	}
}

func (sc *Compiler) compileSExpr(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	if len(e) == 0 {
		return EB{}.ProductType(), nil
	}
	e = filterComments(e)
	op, args := e[0], e[1:]
	opLoc := append(loc, 0)
	switch op := op.(type) {
	case ast.Symbol:
		// intercept macros
		if fn, exists := sc.macros[op]; exists {
			e2, err := fn(args)
			if err != nil {
				return nil, err
			}
			return sc.compileAST(ctx, eb, opLoc, scope, nil, e2)
		}
	case ast.Op:
		// intercept built-ins
		if fn, exists := sc.builtIns[op]; exists {
			return fn(ctx, eb, loc, scope, args)
		}
		// intercept primitives
		if op, exists := sc.prims[op]; exists {
			return sc.compilePrim(ctx, eb, loc, scope, op, args)
		}
		return nil, fmt.Errorf("invalid primitive %v", op)
	}
	opExpr, err := sc.compileAST(ctx, eb, opLoc, scope, nil, op)
	if err != nil {
		return nil, err
	}
	if opExpr.IsLiteral() && myc.Supersets(MacroType, opExpr.Value().Type()) {
		argVals := slices2.Map(args, ASTToMycelium)
		argExprs := slices2.Map(argVals, eb.Lit)
		macroOut, err := eval[myc.Value](ctx, sc.s, sc.vm, eb.Apply(opExpr, eb.List(argExprs...)))
		if err != nil {
			return nil, err
		}
		e2, err := ASTFromMycelium(macroOut)
		if err != nil {
			return nil, err
		}
		return sc.compileAST(ctx, eb, loc, scope, nil, e2)
	} else {
		argExprs := []*Expr{}
		for i := range args {
			loc2 := append(loc, uint32(1+i))
			expr, err := sc.compileAST(ctx, eb, loc2, scope, nil, args[i])
			if err != nil {
				return nil, err
			}
			argExprs = append(argExprs, expr)
		}
		return eb.Apply(opExpr, eb.Product(argExprs...)), nil
	}
}

func (sc *Compiler) compileCompound(ctx context.Context, eb EB, loc Loc, scope *Scope, tys []myc.Type, e []ast.Node) ([]*Expr, error) {
	if len(tys) != len(e) {
		panic(len(tys))
	}
	var exprs []*Expr
	for i := range e {
		loc2 := append(loc, uint32(i))
		expr, err := sc.compileAST(ctx, eb, loc2, scope, tys[i], e[i])
		if err != nil {
			return nil, err
		}
		if expr != nil {
			exprs = append(exprs, expr)
		}
	}
	return exprs, nil
}

func (sc *Compiler) compileArray(ctx context.Context, eb EB, loc Loc, scope *Scope, ty myc.Type, e ast.Array) (*Expr, error) {
	tys := make([]myc.Type, len(e))
	var elemTy *Expr
	if ty != nil {
		if at, ok := ty.(*myc.ArrayType); ok {
			elemTy = lit(at.Elem())
			for i := range tys {
				tys[i] = at.Elem()
			}
		} else {
			return nil, fmt.Errorf("cannot coerce %v into type %v", e, ty)
		}
	}
	exprs, err := sc.compileCompound(ctx, eb, loc, scope, tys, e)
	if err != nil {
		return nil, err
	}
	if elemTy == nil && len(e) == 0 {
		elemTy = lit(myc.Bottom())
	} else if elemTy == nil {
		elemTy = EB{}.TypeOf(exprs[0])
	}
	return EB{}.Array(elemTy, exprs...), nil
}

func (sc *Compiler) compileTuple(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.Tuple) (*Expr, error) {
	tys := make([]myc.Type, len(e))
	exprs, err := sc.compileCompound(ctx, eb, loc, scope, tys, e)
	if err != nil {
		return nil, err
	}
	return EB{}.Product(exprs...), nil
}

func (sc *Compiler) compileInt(eb EB, ty myc.Type, x ast.Int) (*Expr, error) {
	bi := x.BigInt()
	var bs []myc.Bit
	for i := 0; i < bi.BitLen(); i++ {
		bs = append(bs, myc.Bit(bi.Bit(i)))
	}
	switch ty := ty.(type) {
	case nil:
		if bi.BitLen() == 0 || bi.BitLen() == 1 {
			return eb.Bit(myc.Bit(bi.Bit(0))), nil
		}
		return lit(myc.NewBitArray(bs...)), nil
	case myc.BitType:
		if bi.BitLen() == 0 {
			return lit(myc.NewBit(0)), nil
		}
		return lit(myc.NewBit(bi.Bit(0))), nil
	case *myc.ArrayType:
		if !myc.Equal(ty.Elem(), myc.BitType{}) {
			break
		}
		for len(bs) < int(ty.Len()) {
			bs = append(bs, 0)
		}
		return lit(myc.NewBitArray(bs...)), nil
	}
	return nil, fmt.Errorf("canot compile int into ty %v", ty)
}

// compilePrim compiles a primitive expression
func (sc *Compiler) compilePrim(ctx context.Context, eb mycexpr.EB, loc Loc, scope *Scope, code spec.Op, args []ast.Node) (*Expr, error) {
	if len(args) < code.InDegree() {
		return nil, fmt.Errorf("too few args to %v. HAVE: %v", code, args)
	}
	tys := make([]myc.Type, len(args))
	switch code {
	case spec.MakeSum:
		tys[1] = myc.SizeType()
	case spec.Field, spec.Slot:
		tys[1] = myc.SizeType()
	case spec.Concat:
		tys[0] = myc.ArrayOf(myc.BitType{}, 1)
		tys[1] = tys[0]
	case spec.Craft:
		if len(args) > 1 {
			if a, ok := args[1].(ast.Array); ok {
				tys[1] = myc.ArrayOf(myc.AnyTypeType{}, len(a))
			}
		}
	}

	args2 := make([]*Expr, len(args))
	for i := range args {
		a, err := sc.compileAST(ctx, eb, loc, scope, tys[i], args[i])
		if err != nil {
			return nil, err
		}
		args2[i] = a
	}
	return mycexpr.NewExpr(code, args2...)
}

// def is the def built-in
func (sc *Compiler) def(ctx context.Context, eb mycexpr.EB, loc Loc, scope *Scope, sexpr ast.SExpr) (*mycexpr.Expr, error) {
	if len(sexpr) != 2 {
		return nil, fmt.Errorf("def takes 2 arguments. HAVE %v", sexpr)
	}
	k, ok := sexpr[0].(ast.Symbol)
	if !ok {
		return nil, fmt.Errorf("first argument to def must be symbol")
	}
	expr, err := sc.compileAST(ctx, eb, loc, scope, nil, sexpr[1])
	if err != nil {
		return nil, err
	}
	if !scope.Put(string(k), expr) {
		return nil, fmt.Errorf("symbol (%s) is already defined", k)
	}
	return nil, nil
}

// comptime is the comptime built-in
func (sc *Compiler) comptime(ctx context.Context, eb mycexpr.EB, loc Loc, scope *Scope, e ast.SExpr) (*mycexpr.Expr, error) {
	if len(e) != 1 {
		return nil, fmt.Errorf("compile takes 1 arg")
	}
	expr, err := sc.compileAST(ctx, eb, loc, scope, nil, e[0])
	if err != nil {
		return nil, err
	}
	// This should be the only call to vm.Eval
	val, err := eval[myc.Value](ctx, sc.s, sc.vm, expr)
	if err != nil {
		return nil, err
	}
	return mycexpr.Literal(val), nil
}

// scope is the scope built-in
func (sc *Compiler) scope(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	return sc.do(ctx, eb, loc, scope.Child(nil), e)
}

// do is the do built-in
func (sc *Compiler) do(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	var exprs []*Expr
	for i := range e {
		expr, err := sc.compileAST(ctx, eb, loc, scope, nil, e[i])
		if err != nil {
			return nil, err
		}
		if expr != nil {
			exprs = append(exprs, expr)
		}
	}
	switch len(exprs) {
	case 0:
		return nil, fmt.Errorf("expression has no value to return: %v", e)
	case 1:
		return exprs[0], nil
	default:
		return eb.Field(eb.Product(exprs...), len(exprs)-1), nil
	}
}

// ref is the ref built-in
func (sc *Compiler) ref(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("ref takes 2 arguments. HAVE: %v", e)
	}
	tyExpr, err := sc.comptime(ctx, eb, append(loc, 0), scope, ast.SExpr{e[0]})
	if err != nil {
		return nil, err
	}
	ty, err := eval[myc.Type](ctx, sc.s, sc.vm, tyExpr)
	if err != nil {
		return nil, err
	}
	utref, ok := e[1].(ast.Ref)
	if !ok {
		return nil, fmt.Errorf("reg arg[1] must be ast.Ref")
	}
	return lit(myc.NewRef(ty, utref)), nil
}

func (sc *Compiler) self(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	if len(e) != 0 {
		return nil, fmt.Errorf("self takes 0 arguments")
	}
	return mycexpr.Self(), nil
}

func (sc *Compiler) kind(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	if len(e) < 1 {
		return nil, fmt.Errorf("kind takes >= 1 argument")
	}
	n, ok := e[0].(ast.Int)
	if !ok {
		return nil, fmt.Errorf("kind arg0 must be ast.Int")
	}
	kindCode := spec.TypeCode(n.BigInt().Uint64())
	switch kindCode {
	case spec.TC_Kind:
		return lit(myc.KindKind()), nil
	case spec.TC_Bit:
		return lit(myc.BitKind()), nil
	case spec.TC_Array:
		return lit(myc.ArrayKind()), nil
	case spec.TC_List:
		return lit(myc.ListKind()), nil
	case spec.TC_Ref:
		return lit(myc.RefKind()), nil
	case spec.TC_Sum:
		if len(e) != 2 {
			return nil, fmt.Errorf("kind Sum requires 2 args")
		}
		n, ok := e[1].(ast.Int)
		if !ok {
			return nil, fmt.Errorf("kind arg1 must be ast.Int")
		}
		return lit(myc.SumKind(int(n.BigInt().Int64()))), nil
	case spec.TC_Product:
		if len(e) != 2 {
			return nil, fmt.Errorf("kind Product requires 2 args")
		}
		n, ok := e[1].(ast.Int)
		if !ok {
			return nil, fmt.Errorf("kind arg1 must be ast.Int")
		}
		return lit(myc.ProductKind(int(n.BigInt().Int64()))), nil
	case spec.TC_Prog:
		return lit(myc.ProgKind()), nil
	case spec.TC_Lazy:
		return lit(myc.LazyKind()), nil
	case spec.TC_Lambda:
		return lit(myc.LambdaKind()), nil
	case spec.TC_Fractal:
		return lit(myc.FractalKind()), nil
	case spec.TC_Port:
		return lit(myc.PortKind()), nil
	case spec.TC_Distinct:
		return lit(myc.DistinctKind()), nil
	case spec.TC_AnyProg:
		panic("TC_AnyProg")
	case spec.TC_AnyType:
		return lit(myc.AnyTypeKind()), nil
	case spec.TC_AnyValue:
		return lit(myc.AnyValueKind()), nil
	default:
		return nil, fmt.Errorf("(kind %v)", kindCode)
	}
}

func (sc *Compiler) macro(ctx context.Context, eb EB, loc Loc, scope *Scope, e ast.SExpr) (*Expr, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("macro requires 2 arguments. HAVE: %v", e)
	}

	innerScope := make(map[ast.Symbol]ast.Node)
	switch args := e[0].(type) {
	case ast.Array:
		for i := range args {
			sym, ok := args[i].(ast.Symbol)
			if !ok {
				return nil, fmt.Errorf("macro: arg0 must be Array of Symbols")
			}
			innerScope[sym] = ast.SExpr{ast.Op("slot"), ast.Param(0), ast.NewInt(i)}
		}
	case ast.Symbol:
		innerScope[args] = ast.Param(0)
	default:
		return nil, fmt.Errorf("macro: arg0 must be ast.Array or Symbol")
	}

	body := ast.SExpr{ast.Op("scope")}
	for sym, val := range innerScope {
		body = append(body, Def(sym, val))
	}
	body = append(body, e[1:]...)
	bodyExpr, err := sc.compileAST(ctx, eb, loc, scope, AST_Node, body)
	if err != nil {
		return nil, err
	}
	la, err := myc.NewLambda(myc.ListOf(AST_Node), AST_Node, bodyExpr.Build())
	if err != nil {
		return nil, err
	}
	return lit(la), nil
}

func isDef(e ast.Node) bool {
	se, ok := e.(ast.SExpr)
	return ok && se.HasPrefix(ast.Op("def"))
}

func isPub(e ast.Node) bool {
	se, ok := e.(ast.SExpr)
	return ok && se.HasPrefix(ast.Op("pub"))
}

func mount(dst map[string]Value, prefix string, ns map[string]Value) {
	for k, v := range ns {
		dst[prefix+"."+k] = v
	}
}

func lit(x Value) *Expr {
	return mycexpr.Literal(x)
}

func filterComments(e []ast.Node) []ast.Node {
	return slices2.Filter(e, func(x ast.Node) bool {
		_, isComment := x.(ast.Comment)
		return !isComment
	})
}

func eval[T myc.Value](ctx context.Context, s cadata.Store, vm *mvm1.VM, x *Expr) (ret T, _ error) {
	vm.Reset()
	laz, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb EB) *Expr {
		return eb.AnyValueFrom(x)
	})
	if err != nil {
		return ret, err
	}
	if err := vm.ImportLazy(ctx, s, laz); err != nil {
		return ret, err
	}
	vm.SetEval()
	vm.Run(ctx, math.MaxUint64)
	if err := vm.Err(); err != nil {
		return ret, err
	}
	av, err := vm.ExportAnyValue(ctx, s)
	if err != nil {
		return ret, err
	}
	anyval, err := myc.LoadRoot(ctx, s, av.AsBytes())
	if err != nil {
		return ret, err
	}
	ret, ok := anyval.Unwrap().(T)
	if !ok {
		return ret, fmt.Errorf("eval: bad result %v", anyval)
	}
	return ret, nil
}

func mapMap[K1 comparable, V1 any, K2 comparable, V2 any](x map[K1]V1, fn func(K1, V1) (K2, V2)) map[K2]V2 {
	y := make(map[K2]V2)
	for k, v := range x {
		k2, v2 := fn(k, v)
		y[k2] = v2
	}
	return y
}
