package compile

import (
	"myceliumweb.org/mycelium/mycexpr"
)

type Scope struct {
	Parent *Scope
	NS     map[string]*Expr
}

func (s *Scope) find(k string) (*Expr, uint32) {
	var n uint32
	for ; s != nil; s = s.Parent {
		if v, exists := s.NS[k]; exists {
			return v, n
		}
		n++
	}
	return nil, n
}

func (s *Scope) Get(k string) *Expr {
	e, n := s.find(k)
	return shiftParams(e, n)
}

// Put returns true if a new value was successfully created.
func (s *Scope) Put(k string, x *Expr) bool {
	if s.NS == nil {
		s.NS = make(map[string]*Expr)
	}
	if _, exists := s.NS[k]; exists {
		return false
	}
	s.NS[k] = x
	return true
}

func (s *Scope) Child(ns map[string]*Expr) *Scope {
	return &Scope{Parent: s, NS: ns}
}

func shiftParams(e *Expr, n uint32) *Expr {
	return e.MapLeaves(func(e *Expr) *Expr {
		if e.IsParam() {
			return mycexpr.Param(e.Param() + n)
		}
		return e
	})
}
