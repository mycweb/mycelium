package test

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spore/compile"
)

var (
	// TestEnvType is the type passed to the test
	TestEnvType    = myc.NewPortType(myc.SumType{}, myc.SumType{}, myc.SumType{}, myc.SumType{})
	TestLambdaType = myc.NewLambdaType(TestEnvType, myc.ProductType{})
)

type Test struct {
	// Pkg is the package the test is from
	Pkg *compile.Package
	// Name is the test name as it is declared in the package namespace
	Name string
	// Lambda is the Lambda representing the test procedure
	Lambda *myc.Lambda
}

type Result struct {
	Test Test
	Pass bool

	Err   error
	Fault myc.Value
}

// List lists the tests in a package
func List(pkg compile.Package) ([]Test, error) {
	ns := pkg.NS
	keys := slices.Collect(maps.Keys(ns))
	slices.Sort(keys)

	var tests []Test
	for _, key := range keys {
		val := ns[key]
		if !strings.HasPrefix(string(key), "Test") {
			continue
		}
		if !myc.TypeContains(TestLambdaType, val) {
			return nil, fmt.Errorf("names starting with Test must be tests. %v: %v", key, val)
		}
		tests = append(tests, Test{
			Pkg:    &pkg,
			Name:   string(key),
			Lambda: val.(*myc.Lambda),
		})
	}
	return tests, nil
}

// Run runs the Test x and returns a result
func Run(ctx context.Context, src cadata.Getter, x Test) (Result, error) {
	s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	vm := mvm1.New(0, s, mvm1.DefaultAccels())
	port := myc.NewRandPort(TestEnvType)
	vm.PutPort(mvm1.PortFromBytes(port.Data()), mvm1.PortBackend{})
	laz, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb mycexpr.EB) *mycexpr.Expr {
		return eb.Apply(eb.Lit(x.Lambda), eb.Lit(port))
	})
	if err != nil {
		return Result{}, err
	}
	if err := vm.ImportLazy(ctx, src, laz); err != nil {
		return Result{}, err
	}
	vm.SetEval()
	vm.Run(ctx, math.MaxUint64)
	if err := vm.Err(); err != nil {
		return Result{Test: x, Pass: false, Err: err, Fault: vm.GetFault()}, nil
	}
	return Result{
		Test: x,
		Pass: true,
	}, nil
}
