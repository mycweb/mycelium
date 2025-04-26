package ast

import (
	"fmt"
	"math/big"
	"strings"
)

type Node interface {
	isNode()
}

type SExpr []Node

func (SExpr) isNode() {}

func (e SExpr) String() string {
	var parts []string
	for i := range e {
		parts = append(parts, fmt.Sprint(e[i]))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func (e SExpr) HasPrefix(prefix ...Node) bool {
	if len(e) < len(prefix) {
		return false
	}
	for i := range prefix {
		if e[i] != prefix[i] {
			return false
		}
	}
	return true
}

type Int struct {
	bi *big.Int
}

func NewBigInt(x *big.Int) Int {
	x2 := new(big.Int)
	x2.Set(x)
	return Int{x2}
}

func NewInt64(x int64) Int {
	bi := new(big.Int)
	bi.SetInt64(x)
	return NewBigInt(bi)
}

func NewUInt64(x uint64) Int {
	bi := new(big.Int)
	bi.SetUint64(x)
	return NewBigInt(bi)
}

func NewInt(x int) Int {
	return NewInt64(int64(x))
}

func (Int) isNode() {}

func (i Int) BigInt() *big.Int {
	return i.bi
}

func (i Int) String() string {
	return i.bi.String()
}

type String string

func (String) isNode() {}

func (x String) String() string {
	return `"` + string(x) + `"`
}

type Array []Node

func (Array) isNode() {}

type Tuple []Node

func (Tuple) isNode() {}

func (e Tuple) String() string {
	var parts []string
	for i := range e {
		parts = append(parts, fmt.Sprint(e[i]))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

type Table []Row

func (Table) isNode() {}

type Row struct {
	Key   Node
	Value Node
}

func (Row) isNode() {}

type Symbol string

func (Symbol) isNode() {}

type Ref [32]byte

func (Ref) isNode() {}

// Prim is a primitive operation
type Op string

func (Op) isNode() {}
func (p Op) String() string {
	return "!" + string(p)
}

type Param uint32

func (Param) isNode() {}

func (p Param) String() string {
	return fmt.Sprintf("%%%d", p)
}

type Quote struct {
	X Node
}

func (Quote) isNode() {}

func (q Quote) String() string {
	return fmt.Sprintf("'%v", q.X)
}

type Comment string

func (Comment) isNode() {}

func (c Comment) String() string {
	return fmt.Sprintf(";;%s\n", string(c))
}
