package printer

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"myceliumweb.org/mycelium/spore/ast"

	"myceliumweb.org/mycelium/internal/cadata"
)

type AST = ast.Node

// Writer is used by the Print functions
type Writer interface {
	io.Writer
	io.StringWriter
	io.ByteWriter
}

type Printer struct {
	// Deref cause the printer to resolve references when it is set.
	Deref  func(ast.Ref) (ast.Node, error)
	Indent string
}

func (p Printer) PrintString(x AST) string {
	sb := strings.Builder{}
	if err := p.Print(&sb, x); err != nil {
		return err.Error()
	}
	return sb.String()
}

func (p Printer) Print(w Writer, x AST) error {
	return p.printExpr(w, x)
}

func (p Printer) printExpr(w Writer, e AST) error {
	switch e := e.(type) {
	case ast.SExpr:
		if _, err := w.WriteString("("); err != nil {
			return err
		}
		for i := range e {
			if i > 0 {
				if _, err := w.WriteString(" "); err != nil {
					return err
				}
			}
			if err := p.printExpr(w, e[i]); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(")"); err != nil {
			return err
		}
		return nil
	case ast.Array:
		a := e
		if _, err := w.WriteString("["); err != nil {
			return err
		}
		for i, v := range a {
			if i > 0 {
				if _, err := w.WriteString(" "); err != nil {
					return err
				}
			}
			if err := p.printExpr(w, v); err != nil {
				return err
			}
		}
		if _, err := w.WriteString("]"); err != nil {
			return err
		}
		return nil
	case ast.Tuple:
		m := e
		if _, err := w.WriteString("{"); err != nil {
			return err
		}
		for i, v := range m {
			if i > 0 {
				if _, err := w.WriteString(" "); err != nil {
					return err
				}
			}
			if err := p.printExpr(w, v); err != nil {
				return err
			}
		}
		if _, err := w.WriteString("}"); err != nil {
			return err
		}
		return nil
	case ast.Table:
		if _, err := w.WriteString("{"); err != nil {
			return err
		}
		for _, row := range e {
			if err := p.printExpr(w, row.Key); err != nil {
				return err
			}
			w.WriteString(": ")
			if err := p.printExpr(w, row.Value); err != nil {
				return err
			}
			w.WriteString(",\n")
		}
		if _, err := w.WriteString("}"); err != nil {
			return err
		}
		return nil
	case ast.Quote:
		w.WriteString("'")
		return p.printExpr(w, e.X)

	// Leaves
	case ast.Symbol:
		_, err := w.WriteString(string(e))
		return err
	case ast.Int:
		_, err := w.WriteString(e.BigInt().String())
		return err
	case ast.String:
		_, err := fmt.Fprintf(w, "%s", e)
		return err
	case ast.Ref:
		enc := base64.NewEncoding(cadata.Base64Alphabet)
		_, err := w.WriteString(enc.EncodeToString(e[:]))
		return err
	case ast.Param:
		_, err := fmt.Fprintf(w, "%v", e)
		return err
	case ast.Op:
		_, err := fmt.Fprintf(w, "%v", e)
		return err
	default:
		panic(e)
	}
}
