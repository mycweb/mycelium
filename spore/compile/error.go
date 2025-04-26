package compile

import (
	"fmt"
)

type Error struct {
	Source *SourceFile
	Loc    Loc
	Cause  error
}

func (e Error) Error() string {
	span := e.Source.Find(e.Loc).Bound
	return fmt.Sprintf("%q:%v: %v", e.Source.Filename, span, e.Cause)
}
