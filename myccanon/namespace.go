package myccanon

import (
	"fmt"
	"slices"
	"strings"

	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"

	"golang.org/x/exp/maps"
)

type (
	Type        = myc.Type
	Value       = myc.Value
	Product     = myc.Product
	ProductType = myc.ProductType
)

var (
	NS_Type      = myc.ListOf(NS_EntryType)
	NS_EntryType = ProductType{
		myc.StringType(),
		myc.AnyValueType{},
	}
)

var (
	_ myc.ConvertableFrom = Namespace{}
	_ myc.ConvertableTo   = Namespace{}
)

type Namespace map[string]Value

// LetNS wraps x in a let clause for ns
func LetNS(ns Namespace, body func(eb EB) *Expr) *Expr {
	eb := EB{}
	return eb.Let(
		mycexpr.Literal(ns.ToMycelium()),
		body,
	)
}

func (ns Namespace) MyceliumType() myc.Type {
	return NS_Type
}

func (ns Namespace) ToMycelium() myc.Value {
	var vs []Value
	syms := maps.Keys(ns)
	slices.Sort(syms)
	for _, sym := range syms {
		v := ns[sym]
		vs = append(vs, Product{myc.NewString(sym), myc.NewAnyValue(v)})
	}
	return myc.NewList(NS_EntryType, vs...)
}

func (ns Namespace) FromMycelium(x myc.Value) error {
	clear(ns)
	arr, ok := x.(*myc.List)
	if !ok {
		return fmt.Errorf("cannot convert %v: %v into Namespace", x, x.Type())
	}
	at := arr.Type().(*myc.ListType)
	if !myc.Supersets(NS_EntryType, at.Elem()) {
		return fmt.Errorf("array of wrong type %v", at.Elem())
	}
	for i := 0; i < arr.Len(); i++ {
		tup := arr.Get(i).(Product)
		k := AsString(tup[0])
		ns[k] = tup[1].(*myc.AnyValue).Unwrap()
	}
	return nil
}

// SetEntrypoint sets the entry in namespace for the empty string to whatever is at the entry for k.
func SetEntrypoint(ns Namespace, k string) {
	if _, exists := ns[k]; !exists {
		panic(fmt.Sprintf("no entry for %v in namespace", k))
	}
	ns[""] = ns[k]
}

func GetEntrypoint(ns Namespace) (Value, error) {
	v, exists := ns[""]
	if !exists {
		return nil, fmt.Errorf("namespace has no entrypoint")
	}
	return v, nil
}

func NSPretty(ns Namespace) string {
	sb := &strings.Builder{}
	sb.WriteString("Namespace{\n")
	ks := maps.Keys(ns)
	for _, k := range ks {
		v := ns[k]
		fmt.Fprintf(sb, "%q: %v\n", k, v)
	}
	sb.WriteString("}")
	return sb.String()
}

var (
	// NSEntry_Key is a Lambda that returns the key for an NS entry
	NSEntry_Key = lambda(NS_EntryType, myc.StringType(), func(eb EB) *Expr {
		return eb.Field(eb.P(0), 0)
	})
	// NSEntry_Value is a Lambda that returns the value for an NS entry
	NSEntry_Value = lambda(NS_EntryType, myc.AnyValueType{}, func(eb EB) *Expr {
		return eb.Field(eb.P(0), 1)
	})
	// NS_Len returns the number of entries in a Namespace
	NS_Len = lambda(NS_Type, myc.SizeType(), func(eb EB) *Expr {
		return eb.ListLen(eb.P(0))
	})
	// NS_Find returns the number of entries
	NS_Find = lambda(ProductType{NS_Type, myc.StringType()}, myc.SizeType(), func(eb EB) *Expr {
		return eb.Apply(
			eb.Lambda(
				myc.ProductType{NS_Type, myc.StringType(), myc.B32Type()},
				myc.B32Type(),
				func(eb EB) *Expr {
					return eb.If(
						expr(spec.Equal,
							eb.Arg(0, 1),
							entryKeyExpr(
								expr(spec.Slot, eb.Arg(0, 0), eb.Arg(0, 2)),
							),
						),
						eb.Arg(0, 2),
						eb.Apply(eb.Self(), eb.Product(
							eb.Arg(0, 0),
							eb.Arg(0, 1),
							eb.Apply(eb.Lit(B32_Add), eb.Product(eb.Arg(0, 2), eb.B32(1))),
						)),
					)
				},
			),
			eb.Product(eb.Arg(0, 0), eb.Arg(0, 1), eb.B32(0)),
		)
	})
	NS_Get = lambda(ProductType{NS_Type, myc.StringType()}, myc.AnyValueType{}, func(eb EB) *Expr {
		return entryValueExpr(eb.Slot(eb.Arg(0, 0), eb.Apply(eb.Lit(NS_Find), eb.P(0))))
	})
)

func NSLenExpr(ns *Expr) *Expr {
	return eb.Apply(eb.Lit(NS_Len), ns)
}

func entryKeyExpr(ent *Expr) *Expr {
	return eb.Apply(eb.Lit(NSEntry_Key), ent)
}

func entryValueExpr(ent *Expr) *Expr {
	return eb.Apply(eb.Lit(NSEntry_Value), ent)
}

// NSGetExpr returns the value of the entry with key=k
func NSGetExpr(ns *Expr, k string) *Expr {
	return eb.Apply(eb.Lit(NS_Get), eb.Product(ns, eb.String(k)))
}

// NSFindExpr finds the first index of an entry with key=k in ns
// The machine faults if the key does not exist
func NSFindExpr(ns *Expr, k string) *Expr {
	return eb.Apply(eb.Lit(NS_Find), eb.Product(ns, eb.String(k)))
}

func AsString(v myc.Value) string {
	return v.(*myc.List).Array().(myc.ByteArray).AsString()
}

func expr(code spec.Op, args ...*Expr) *Expr {
	e, err := mycexpr.NewExpr(code, args...)
	if err != nil {
		panic(err)
	}
	return e
}

func lit(x myc.Value) *Expr {
	return mycexpr.Literal(x)
}

func b32Add(a, b *Expr) *Expr {
	return eb.Apply(eb.Lit(B32_Add), eb.Product(a, b))
}
