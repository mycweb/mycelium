package stdlib

import (
	"embed"

	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycss"
)

//go:embed *
var FS embed.FS

var Base = map[string]myccanon.Namespace{
	"bits":          bitsPkg,
	"floats":        floatsPkg,
	"substrate":     substratePkg,
	"substrate/net": netPkg,
}

var bitsPkg = myccanon.Namespace{
	"NOT": myccanon.NOT,
	"AND": myccanon.AND,
	"OR":  myccanon.OR,
	"XOR": myccanon.XOR,

	"B32":     myccanon.B32,
	"b32_AND": myccanon.B32_AND,
	"b32_OR":  myccanon.B32_OR,
	"b32_NOT": myccanon.B32_NOT,
	"b32_XOR": myccanon.B32_XOR,
	"b32_add": myccanon.B32_Add,
	"b32_sub": myccanon.B32_Sub,
	"b32_mul": myccanon.B32_Mul,
	"b32_div": myccanon.B32_Div,

	"B64":     myccanon.B64,
	"b64_AND": myccanon.B64_AND,
	"b64_OR":  myccanon.B64_OR,
	"b64_NOT": myccanon.B64_NOT,
	"b64_XOR": myccanon.B64_XOR,
	"b64_add": myccanon.B64_Add,
	"b64_sub": myccanon.B64_Sub,
	"b64_mul": myccanon.B64_Mul,
	"b64_div": myccanon.B64_Div,
}

var floatsPkg = myccanon.Namespace{
	"Float16": myccanon.Float16,

	"Float32": myccanon.Float32,
	"f32_add": myccanon.Float32_Add,
	"f32_sub": myccanon.Float32_Sub,
	"f32_mul": myccanon.Float32_Mul,
	"f32_div": myccanon.Float32_Div,

	"Float64": myccanon.Float64,
	"f64_add": myccanon.Float64_Add,
	"f64_sub": myccanon.Float64_Sub,
	"f64_mul": myccanon.Float64_Mul,
	"f64_div": myccanon.Float64_Div,
}

var netPkg = myccanon.Namespace{
	"NodeInfo": mycss.DEV_NET_NodeInfo,
	"Message":  mycss.DEV_NET_Message,
	"Addr":     mycss.DEV_NET_Addr,
}

var substratePkg = myccanon.Namespace{
	"CellDev": mycss.DEV_CELL_Type,
	"NetDev":  mycss.DEV_NET_Node,
}
