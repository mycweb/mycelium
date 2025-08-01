package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type AnyProgType struct{}

func (et AnyProgType) Type() Type {
	return AnyProgKind()
}

func (et AnyProgType) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (et AnyProgType) Encode(BitBuf) {
}

func (et AnyProgType) Decode(BitBuf, LoadFunc) error {
	return nil
}

func (et AnyProgType) Zero() Value {
	pt := ProgType{}
	return NewExpr(*pt.Zero().(*Prog))
}

func (et AnyProgType) SizeOf() int {
	return spec.ExprTypeBits
}

func (et AnyProgType) Components() iter.Seq[Value] {
	return emptyIter
}
func (AnyProgType) isType()  {}
func (AnyProgType) isValue() {}

var _ Value = &Expr{}

// Expr holds a Product[Ref[Prog[_]], ProgType]
// The Ref points to a Prog
// The size holds the size of the program.
type Expr struct {
	prog Prog
	ref  *Ref
}

func NewExpr(prog Prog) *Expr {
	ref := mkRef(&prog)
	return &Expr{
		prog: prog,
		ref:  &ref,
	}
}

func (e *Expr) Prog() Prog {
	return e.prog
}

func (e *Expr) Type() Type {
	return AnyProgType{}
}

func (e *Expr) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	if yes, err := dst.Exists(ctx, &e.ref.cid); err != nil {
		return err
	} else if yes {
		return nil
	}
	if err := e.prog.PullInto(ctx, dst, src); err != nil {
		return err
	}
	if ref2, err := Post(ctx, dst, &e.prog); err != nil {
		return err
	} else if ref2.cid != e.ref.cid {
		panic(ref2.cid)
	}
	return nil
}

func (e *Expr) Encode(bb BitBuf) {
	Product{
		e.ref,
		e.prog.Type(),
	}.Encode(bb)
}

func (e *Expr) Decode(bb BitBuf, load LoadFunc) error {
	refbuf := bb.Slice(0, RefBits)
	sizeBuf := bb.Slice(RefBits, bb.Len())

	var pt ProgType
	if err := pt.Decode(sizeBuf, load); err != nil {
		return err
	}
	rt := NewRefType(&pt)
	ref := rt.Zero().(*Ref)
	if err := ref.Decode(refbuf, load); err != nil {
		return err
	}
	ref = ref.retype(&pt)
	prog, err := load(*ref)
	if err != nil {
		return err
	}
	e.ref = ref
	e.prog = *prog.(*Prog)
	return nil
}

func (e *Expr) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		_ = yield(e.ref) && yield(NewSize(e.prog.Size()))
	}
}

func (*Expr) isValue() {}

func (e *Expr) String() string {
	return fmt.Sprintf("Expr{progLen=%d}", len(e.prog))
}

// ValidateBody checks that the body does not contain any free parameters above level.
// - ValidateBody(_, 0) is called by NewLazy
// - ValidateBody(_, 1) is called by NewLambda
func ValidateBody(body *Expr, level uint32, hasSelf bool) error {
	return validateBody(body.prog, level, hasSelf)
}

func validateBody(body Prog, level uint32, hasSelf bool) error {
	node := body.Root()
	switch {
	case node.IsLiteral():
		return nil
	case node.IsSelf():
		// if !hasSelf {
		// 	return fmt.Errorf("no self in this context")
		// }
		return nil
	case node.IsParam():
		if node.Param() >= level {
			return fmt.Errorf("free parameter %v", node.Param())
		}
		return nil
	case node.IsCode(spec.Lambda):
		if err := validateBody(body.Input(0), level, true); err != nil {
			return err
		}
		if err := validateBody(body.Input(1), level, hasSelf); err != nil {
			return err
		}
		if err := validateBody(body.Input(2), level+1, hasSelf); err != nil {
			return err
		}
		return nil
	case node.IsCode(spec.Let):
		if err := validateBody(body.Input(0), level, hasSelf); err != nil {
			return err
		}
		return validateBody(body.Input(1), level+1, hasSelf)
	case node.IsCode(spec.Fractal):
		return validateBody(body.Input(0), level, true)
	default:
		for arg := range body.Inputs() {
			if err := validateBody(arg, level, hasSelf); err != nil {
				return err
			}
		}
		return nil
	}
}
