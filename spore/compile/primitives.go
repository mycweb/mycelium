package compile

import (
	"strings"
	"unicode"

	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"
)

// Primitives returns a namespace with all of the Mycelium Primitive Operations
func Primitives() map[ast.Op]spec.Op {
	ret := make(map[ast.Op]spec.Op)
	for _, oc := range spec.All() {
		name := oc.String()
		if !isAllCaps(name) {
			name = lowerFirst(name)
		}
		ret[ast.Op(name)] = oc
	}
	return ret
}

func isAllCaps(x string) bool {
	for _, r := range x {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func lowerFirst(x string) string {
	if len(x) == 0 {
		return x
	}
	return strings.ToLower(x[0:1]) + x[1:]
}
