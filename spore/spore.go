package spore

import (
	"bytes"
	"context"
	"maps"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	mycelium "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/compile"
	"myceliumweb.org/mycelium/spore/decompile"
	"myceliumweb.org/mycelium/spore/parser"
	"myceliumweb.org/mycelium/spore/printer"
)

type (
	Value     = mycelium.Value
	Type      = mycelium.Type
	Tuple     = mycelium.Product
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

func Decompile(x mycelium.Value) ast.Node {
	dc := decompile.New(Dictionary())
	return dc.Decompile(x)
}

func PrintString(x mycelium.Value) string {
	p := printer.Printer{}
	return p.PrintString(Decompile(x))
}

// Dictionary returns a dictionary from Fingerprints to values in the standard library
func Dictionary() map[cadata.ID]ast.Node {
	return maps.Clone(decompileDict)
}

var decompileDict = func() map[cadata.ID]ast.Node {
	cid := func(x mycelium.Value) cadata.ID {
		return mycelium.Fingerprint(x)
	}
	ret := map[cadata.ID]ast.Node{}
	for k, v := range preambleNS {
		ret[cid(v.Build())] = ast.Symbol(k)
	}
	return ret
}()
