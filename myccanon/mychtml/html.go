package mychtml

import (
	"context"
	"fmt"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"

	"myceliumweb.org/mycelium/internal/cadata"
)

// HTML is a syntax tree node in an HTML document
type HTML struct {
	Type     string
	Text     string
	Children []HTML

	id cadata.ID
}

func (n *HTML) GetID() cadata.ID {
	if n.id.IsZero() {
		ref, err := PostHTML(context.TODO(), stores.NewTotal(mycelium.Hash, mycelium.MaxSizeBytes), *n)
		if err != nil {
			panic(err)
		}
		n.id = cadata.ID(ref.Bytes()[:32])
	}
	return n.id
}

// HTMLType is the type of an HTML node
func HTMLType() myc.Type {
	rt, err := mycexpr.BuildFractalType(func(eb myccanon.EB) *myccanon.Expr {
		return eb.ProductType(
			mycexpr.Literal(myc.StringType()), // type
			mycexpr.Literal(myc.StringType()), // text
			eb.ListType(eb.RefType(mycexpr.Self())),
		)
	})
	if err != nil {
		panic(err)
	}
	return rt
}

// EncodeHTML encodes an HTML node as a Mycelium Value using s as a store
func EncodeHTML(ctx context.Context, s cadata.PostExister, x HTML) (myc.Value, error) {
	var children []myc.Value
	for _, child := range x.Children {
		ref, err := PostHTML(ctx, s, child)
		if err != nil {
			return nil, err
		}
		children = append(children, &ref)
	}
	return myc.Product{
		myc.NewString(x.Type),
		myc.NewString(x.Text),
		myc.NewList(myc.NewRefType(HTMLType()), children...),
	}, nil
}

// PostHTML encodes and then posts an HTML node to s, and then returns a Mycelium Ref
func PostHTML(ctx context.Context, s cadata.PostExister, x HTML) (myc.Ref, error) {
	val, err := EncodeHTML(ctx, s, x)
	if err != nil {
		return myc.Ref{}, err
	}
	return myc.Post(ctx, s, val)
}

// DecodeHTML converts a mycelium Value into an HTML node.
func DecodeHTML(ctx context.Context, s cadata.Getter, x myc.Value) (HTML, error) {
	if !myc.TypeContains(HTMLType(), x) {
		return HTML{}, fmt.Errorf("cannot convert %v to HTML node", x)
	}
	p := x.(myc.Product)
	var children []HTML
	if err := arrayForEach(p[2], func(x myc.Value) error {
		ref := x.(*myc.Ref)
		node, err := LoadHTML(ctx, s, *ref)
		if err != nil {
			return err
		}
		children = append(children, node)
		return nil
	}); err != nil {
		return HTML{}, err
	}
	return HTML{
		Type:     myccanon.AsString(p[0]),
		Text:     myccanon.AsString(p[1]),
		Children: children,
	}, nil
}

func LoadHTML(ctx context.Context, s cadata.Getter, x myc.Ref) (HTML, error) {
	val, err := myc.Load(ctx, s, x)
	if err != nil {
		return HTML{}, err
	}
	return DecodeHTML(ctx, s, val)
}

func PullHTML(ctx context.Context, dst cadata.PostExister, src cadata.Getter, x myc.Value) error {
	return x.PullInto(ctx, dst, src)
}

func arrayForEach(x myc.Value, fn func(myc.Value) error) error {
	al := x.(myc.ArrayLike)
	for i := 0; i < al.Len(); i++ {
		if err := fn(al.Get(i)); err != nil {
			return err
		}
	}
	return nil
}
