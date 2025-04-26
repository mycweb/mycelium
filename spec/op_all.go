package spec

import (
	"strings"
	"unicode"
)

func All() (ret []Op) {
	for i := 0; i < (1 << OpBits); i++ {
		p := Op(i)
		if strings.HasPrefix(p.String(), "Op(") {
			continue
		}
		if unicode.IsLower(rune(p.String()[0])) {
			continue
		}
		ret = append(ret, p)
	}
	return ret
}

// AllStore contains all the operations that manipulate the store
func AllStore() []Op {
	return []Op{Post, Load}
}
