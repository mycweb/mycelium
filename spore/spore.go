package spore

import (
	"bytes"
	"context"
	"maps"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/compile"
	"myceliumweb.org/mycelium/spore/decompile"
	"myceliumweb.org/mycelium/spore/parser"
	"myceliumweb.org/mycelium/spore/printer"
)

type (
	Value     = mycmem.Value
	Type      = mycmem.Type
	Tuple     = mycmem.Product
	Expr      = mycexpr.Expr
	Namespace = myccanon.Namespace
)

// CompileSnippet parses a value from a byte slice
func CompileSnippet(x []byte) (*mycexpr.Expr, error) {
	p := parser.NewParser(bytes.NewReader(x))
	_, e, err := p.ParseAST()
	if err != nil {
		return nil, err
	}
	sc := compile.New(nil, Preamble())
	return sc.CompileAST(context.TODO(), e)
}

func Decompile(x mycmem.Value) ast.Node {
	dc := decompile.New(Dictionary())
	return dc.Decompile(x)
}

func PrintString(x mycmem.Value) string {
	p := printer.Printer{}
	return p.PrintString(Decompile(x))
}

// Dictionary returns a dictionary from Fingerprints to values in the standard library
func Dictionary() map[mycelium.CID]ast.Node {
	return maps.Clone(decompileDict)
}

var decompileDict = func() map[mycelium.CID]ast.Node {
	cid := func(x mycmem.Value) mycelium.CID {
		return mycmem.Fingerprint(x)
	}
	ret := map[mycelium.CID]ast.Node{}
	for k, v := range preambleNS {
		ret[cid(v.Build())] = ast.Symbol(k)
	}
	return ret
}()
