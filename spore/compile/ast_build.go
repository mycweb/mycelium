package compile

import "myceliumweb.org/mycelium/spore/ast"

func Def(sym ast.Symbol, val ast.Node) ast.Node {
	return ast.SExpr{defPrim, sym, val}
}

func Let(bindings map[ast.Symbol]ast.Node, body ast.Node) ast.Node {
	var pairs ast.SExpr
	for sym, val := range bindings {
		pairs = append(pairs, ast.SExpr{sym, val})
	}
	return ast.SExpr{ast.Symbol("let"), pairs, body}
}

func LetOne(k ast.Symbol, v ast.Node, body ast.Node) ast.Node {
	return ast.SExpr{
		ast.Op("let"),
		v,
		ast.SExpr{scopePrim, Def(k, ast.Param(0)), body},
	}
}

func Lambda(args ast.Table, outType ast.Node, body ...ast.Node) ast.Node {
	return append(ast.SExpr{ast.Symbol("lambda"), args, outType}, body...)
}

func Field(x ast.Node, i uint32) ast.Node {
	return ast.SExpr{ast.Op("field"), x, ast.NewUInt64(uint64(i))}
}

func TypeOf(x ast.Node) ast.Node {
	return ast.SExpr{ast.Op("typeOf"), x}
}

func Defc(sym ast.Symbol, val ast.Node) ast.Node {
	return ast.SExpr{ast.Symbol("defc"), sym, val}
}

func comptimeExpr(e ast.Node) ast.Node {
	return ast.SExpr{ast.Op("comptime"), e}
}
