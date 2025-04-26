package spore

import (
	"context"
	_ "embed"
	"strings"

	"golang.org/x/exp/maps"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spore/compile"
	"myceliumweb.org/mycelium/spore/parser"
)

var (
	SymbolType = mycmem.StringType()
)

func Preamble() map[string]*mycexpr.Expr {
	ns := make(map[string]*mycexpr.Expr)
	maps.Copy(ns, preambleNS)
	return ns
}

var preambleNS map[string]*mycexpr.Expr

//go:embed preamble.sp
var preambleFile string

func init() {
	s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	span, astNodes, err := parser.ReadAll(parser.NewParser(strings.NewReader(preambleFile)))
	if err != nil {
		panic(err)
	}
	sc := compile.New(s, nil)
	pkg, err := sc.Compile(context.TODO(), nil, nil, []compile.SourceFile{
		{
			Filename: "preamble",
			Source:   []byte(preambleFile),
			Nodes:    astNodes,
			Span:     span,
		},
	})
	if err != nil {
		panic(err)
	}
	preambleNS = pkg.Internals
}
