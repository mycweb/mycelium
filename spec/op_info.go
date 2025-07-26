package spec

// Info is information about Operations
type Info struct {
	InDegree int `json:"inDegree"`
}

func (p Op) Info() Info {
	return infos[p]
}

// InDegree returns the number of inputs, an operation with this code takes
func (p Op) InDegree() int {
	n := infos[p].InDegree
	if n < 0 {
		return 0
	}
	return n
}

var infos = func() (ret [1 << 16]Info) {
	m := map[Op]Info{
		Pass:  {1},
		Equal: {2},

		Craft:   {2},
		Uncraft: {1},
		TypeOf:  {1},
		SizeOf:  {1},
		Encode:  {1},
		Decode:  {2},

		Let:    {2},
		Mux:    {2},
		Branch: {3},
		Panic:  {1},

		// Bit
		ZERO: {0},
		ONE:  {0},

		// Composite
		ArrayEmpty:   {1},
		ArrayUnit:    {1},
		ProductEmpty: {0},
		ProductUnit:  {1},
		MakeSum:      {3},

		// Compute
		Lazy:    {1},
		Lambda:  {3},
		Fractal: {1},
		Eval:    {1},
		Apply:   {2},

		// Store
		Post: {1},
		Load: {1},

		// Port
		Input:    {1},
		Output:   {2},
		Interact: {2},

		Gather:   {1},
		ListFrom: {1},
		ListTo:   {2},

		AnyValueFrom: {1},
		AnyValueTo:   {2},
		AnyTypeFrom:  {1},
		AnyTypeTo:    {1},

		// Accessors
		Len:     {1},
		Field:   {2},
		Slot:    {2},
		Section: {3},
		Which:   {1},

		// Array Compute
		Map:    {2},
		Reduce: {2},
		Zip:    {2},
		Fold:   {3},
		Concat: {2},
	}
	for i := range ret {
		ret[i] = Info{-1}
	}
	for k, v := range m {
		ret[k] = v
	}
	return ret
}()
