package mycmem

import "fmt"

type ErrDanglingRef struct {
	Value Value
	Ref   *Ref
}

func (e ErrDanglingRef) Error() string {
	return fmt.Sprintf("posting value=%v yielding ref=%v would create a dangling reference to %v", e.Value, mkRef(e.Value), e.Ref.cid)
}

type ErrFractalType struct {
	Body *Prog
	Msg  string
}

func (e ErrFractalType) Error() string {
	return e.Msg
}
