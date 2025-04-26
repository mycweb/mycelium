package compile

import (
	"fmt"
	"path"

	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/parser"
)

// Loc is a location in the source
type Loc []uint32

type SourceFile struct {
	// Filename is the filename within the directory.
	Filename string
	// Source is the raw bytes which make up this file
	Source []byte
	// Nodes is the parsed content of the file
	Nodes []ast.Node
	// Span is the tree of spans for all the nodes in the file
	Span parser.Span
	// Newlines is the location of all the newlines in the file
	Newlines []uint32
}

func (sf *SourceFile) Find(loc Loc) parser.Span {
	span := sf.Span
	for _, i := range loc {
		span = span.Children[i]
	}
	return span
}

// ScanImports looks for import statements in the SourceFiles nodes.
// It returns all the import statements it finds before another directive, and the number of nodes scanned.
func (sf *SourceFile) ScanImports(start int) (imps []ImportStmt, _ int, _ error) {
	var nonImportFound bool
	for i, node := range sf.Nodes[start:] {
		switch x := node.(type) {
		case ast.Comment:
		case ast.SExpr:
			if !isImportStatement(x) {
				nonImportFound = true
				continue
			}
			if nonImportFound {
				return imps, i, fmt.Errorf("import statements must be before all other statements")
			}
			istmt, err := asImportStmt(x)
			if err != nil {
				return nil, 0, err
			}
			imps = append(imps, istmt)
		default:
			nonImportFound = true
		}
	}
	return imps, len(sf.Nodes), nil
}

// ImportStmt is an SExpr of one of the forms:
// - (import "package-name")
// - (import <importAs> "package-name")
type ImportStmt struct {
	// As prefixes the package symbols in an import
	As ast.Symbol
	// Target is the name of the package to import
	Target string
}

func (is ImportStmt) String() string {
	return fmt.Sprintf("(import %s %q)", is.As, is.Target)
}

func isImportStatement(x ast.Node) bool {
	switch x := x.(type) {
	case ast.SExpr:
		return x.HasPrefix(ast.Symbol("import"))
	default:
		return false
	}
}

func asImportStmt(x ast.Node) (ImportStmt, error) {
	se := x.(ast.SExpr)
	if se[0] != ast.Symbol("import") {
		panic(x)
	}
	switch len(se) {
	case 2:
		target, ok := se[1].(ast.String)
		if !ok {
			return ImportStmt{}, fmt.Errorf("import statement target must be string. HAVE: %v", x)
		}
		return ImportStmt{
			As:     ast.Symbol(path.Base(string(target))),
			Target: string(target),
		}, nil
	case 3:
		as, ok := se[1].(ast.Symbol)
		if !ok {
			return ImportStmt{}, fmt.Errorf("import statement prefix symbol must be symbol. HAVE: %v", x)
		}
		target, ok := se[2].(ast.String)
		if !ok {
			return ImportStmt{}, fmt.Errorf("import statement target must be string. HAVE: %v", x)
		}
		return ImportStmt{
			As:     as,
			Target: string(target),
		}, nil
	default:
		return ImportStmt{}, fmt.Errorf("import statement must have length 2 or 3.  HAVE: %v", x)
	}
}
