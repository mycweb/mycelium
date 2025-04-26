package mvm1

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myctests"
)

func TestVM(t *testing.T) {
	t.Parallel()
	type testCase struct {
		Name  string
		Setup func(t testing.TB, vm *VM)
		Prog  []I

		// End is what is on the stack at the end
		End []Word
	}
	tcs := []testCase{
		{
			Name: "Seq",
			Prog: []I{
				pushB32(11),
				pushB32(22),
				pushB32(33),
			},
			End: []Word{11, 22, 33},
		},
		{
			Name: "PostLoad",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(123)
			},
			Prog: []I{
				postI{salt: nil, inputBits: 32},
				loadI{outputBits: 32},
			},
			End: []Word{123},
		},
		{
			Name: "123 == 123 => 1",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(123)
				vm.push(123)
			},
			Prog: equal(32),
			End:  []Word{1},
		},
		{
			Name: "123 == 124 => 0",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(123)
				vm.push(124)
			},
			Prog: equal(32),
			End:  []Word{0},
		},
		{
			Name: "Concat 2 bits",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(1)
				vm.push(1)
			},
			Prog: []I{
				concatI{leftBits: 1, rightBits: 1},
			},
			End: []Word{3}, // 2 low bits set 2 + 1 = 3
		},
		{
			Name: "Concat [1 1] [1] => [1 1 0]",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(3)
				vm.push(1)
			},
			Prog: []I{
				concatI{leftBits: 2, rightBits: 1},
			},
			End: []Word{7},
		},
		{
			Name: "Branch 0",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(0)
			},
			Prog: mkBranch(
				[]I{pushB32(777)},
				[]I{pushB32(333)},
			),
			End: []Word{777},
		},
		{
			Name: "Branch 1",
			Setup: func(t testing.TB, vm *VM) {
				vm.push(1)
			},
			Prog: mkBranch(
				[]I{pushB32(777)},
				[]I{pushB32(333)},
			),
			End: []Word{333},
		},
		{
			Name: "Lazy Literal",
			Setup: func(t testing.TB, vm *VM) {
				laz, err := mycexpr.BuildLazy(myc.B32Type(), func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.B32(123)
				})
				require.NoError(t, err)
				require.NoError(t, vm.ImportLazy(context.TODO(), vm.store, laz))
				vm.SetEval() // TODO
			},
			End: []Word{123},
		},
		{
			Name: "Let %0",
			Setup: func(t testing.TB, vm *VM) {
				laz, err := mycexpr.BuildLazy(myc.B32Type(), func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.Let(
						eb.B32(123),
						func(eb mycexpr.EB) *mycexpr.Expr { return eb.P(0) },
					)
				})
				require.NoError(t, err)
				require.NoError(t, vm.ImportLazy(context.TODO(), vm.store, laz))
				vm.SetEval() // TODO
			},
			End: []Word{123},
		},
		{
			Name: "Lambda",
			Setup: func(t testing.TB, vm *VM) {
				ctx := context.TODO()
				laz, err := mycexpr.BuildLazy(myc.ProductType{}, func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.Apply(mycexpr.EB{}.Lambda(
						myc.ProductType{},
						myc.ProductType{},
						func(eb mycexpr.EB) *mycexpr.Expr { return eb.Product() },
					),
						eb.Product(),
					)
				})
				require.NoError(t, err)
				require.NoError(t, vm.ImportLazy(ctx, vm.store, laz))
				vm.SetEval()
			},
			End: []Word{},
		},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d/%s", i, tc.Name), func(t *testing.T) {
			ctx := testutil.Context(t)
			s := testutil.NewStore(t)
			vm := New(100, s, nil)
			vm.prog = tc.Prog
			if tc.Setup != nil {
				tc.Setup(t, vm)
			}
			stepsTaken := vm.Run(ctx, 1e6)
			t.Log("steps taken:", stepsTaken)
			require.NoError(t, vm.err)
			require.Equal(t, tc.End, vm.stack)
		})
	}
}

func pushB32(x uint32) pushI {
	return pushI{x: x}
}

func TestImportExport(t *testing.T) {
	t.Parallel()
	src := testutil.NewStore(t)
	vals := myctests.InterestingValues(src)
	for i, val := range vals {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			ctx := testutil.Context(t)
			vm := New(100, testutil.NewStore(t), nil)

			data1, err := myc.SaveRoot(ctx, src, myc.NewAnyValue(val))
			require.NoError(t, err)
			av1 := AnyValue{}
			av1.FromBytes(data1)
			require.NoError(t, vm.ImportAnyValue(ctx, src, av1))

			dst := testutil.NewStore(t)
			av2, err := vm.ExportAnyValue(ctx, dst)
			require.NoError(t, err)
			require.Equal(t, av1, av2)
			data2 := av2.AsBytes()

			require.Equal(t, data1, data2)
		})
	}
}

func TestEvalSuite(t *testing.T) {
	t.Parallel()
	src := testutil.NewStore(t)
	tcs := myctests.EvalVecs(src)
	tcs = append(tcs, myctests.EvalVecs2(src)...)

	for i, tc := range tcs {
		tc := tc
		name := fmt.Sprintf("%d/%v", i, tc.I)
		if tc.Name != "" {
			name = fmt.Sprintf("%d/%s", i, tc.Name)
		}
		t.Run(name, func(t *testing.T) {
			ctx := testutil.Context(t)
			s := testutil.NewStore(t)
			vm := New(100, s, DefaultAccels())
			laz, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb mycexpr.EB) *mycexpr.Expr {
				return eb.AnyValueFrom(tc.I)
			})
			require.NoError(t, err)
			require.NoError(t, vm.ImportLazy(ctx, src, laz))
			vm.SetEval()
			steps := vm.Run(ctx, 1e4)
			t.Log("steps", steps)
			require.NoError(t, vm.Err())
			require.Equal(t, AnyValueBits/WordBits, len(vm.stack), "wrong stack size")
			actual, err := vm.ExportAnyValue(ctx, s)
			require.NoError(t, err)
			actualBytes := make([]byte, len(actual)*4)
			wordsToBytes(actual[:], actualBytes)

			expectedBytes, err := myc.SaveRoot(ctx, s, myc.NewAnyValue(tc.O))
			require.NoError(t, err)
			if !bytes.Equal(expectedBytes, actualBytes) {
				t.Errorf("HAVE: %x\nWANT: %x\n", actualBytes, expectedBytes)
				actual, err := myc.LoadRoot(ctx, s, actualBytes)
				require.NoError(t, err)
				t.Logf("HAVE: %v \nWANT: %v\n", actual, tc.O)
			}
		})
	}
}

func TestGolden(t *testing.T) {
	t.Parallel()
	type testCase struct {
		I *mycexpr.Expr
		O myctests.BP
	}
	lit := func(x myc.Value) *mycexpr.Expr {
		return mycexpr.Literal(x)
	}
	post := func(x *mycexpr.Expr) *mycexpr.Expr {
		return mycexpr.EB{}.Post(x)
	}
	postV := func(x myc.Value) *mycexpr.Expr {
		return mycexpr.EB{}.Post(lit(x))
	}
	anyType := func(x myc.Type) *mycexpr.Expr {
		return mycexpr.EB{}.AnyTypeFrom(lit(x))
	}
	exprType0 := &myc.ProgType{}
	tcs := []testCase{
		{I: lit(myc.KindKind()), O: myctests.BP_KindKind},
		{I: lit(myc.BitKind()), O: myctests.BP_BitKind},

		// Lower Kinded Types
		{I: lit(myc.BitType{}), O: myctests.BP_BitType},
		{I: lit(&myc.ProgType{}), O: myctests.BP_ProgType_0},
		{I: lit(myc.SumType{}), O: myctests.BP_SumType_0},
		{I: lit(myc.ProductType{}), O: myctests.BP_ProductType_0},
		{I: lit(myc.AnyTypeType{}), O: myctests.BP_AnyTypeType},
		{I: lit(myc.AnyValueType{}), O: myctests.BP_AnyValueType},

		// Refs to Lower Kinded Types
		{I: postV(myc.BitType{}), O: myctests.BP_Ref_BitType},
		{I: postV(exprType0), O: myctests.BP_Ref_ProgType_0},
		{I: postV(myc.SumType{}), O: myctests.BP_Ref_SumType_0},
		{I: postV(myc.ProductType{}), O: myctests.BP_Ref_ProductType_0},
		{I: postV(myc.AnyTypeType{}), O: myctests.BP_Ref_AnyTypeType},
		{I: postV(myc.AnyValueType{}), O: myctests.BP_Ref_AnyValueType},

		{I: anyType(myc.KindKind()), O: myctests.BP_AnyType_KindKind},
		{I: anyType(myc.BitType{}), O: myctests.BP_AnyType_BitType},

		{I: postV(myc.NewBit(0)), O: myctests.BP_Ref_Bit_0},
		{I: postV(myc.NewBit(1)), O: myctests.BP_Ref_Bit_1},
		{I: post(postV(myc.NewBit(0))), O: myctests.BP_Ref_Ref_Bit_0},
		{I: post(postV(myc.NewBit(1))), O: myctests.BP_Ref_Ref_Bit_1},
	}
	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d/%v", i, tc.I), func(t *testing.T) {
			ctx := testutil.Context(t)
			s := testutil.NewStore(t)
			vm := New(100, s, nil)
			// TODO: figure out how to infer the type here instead of using Bottom
			// Bottom doesn't superset anything, so this will be a type error if/when NewLazy checks types.
			laz, err := mycexpr.BuildLazy(myc.Bottom(), func(eb mycexpr.EB) *mycexpr.Expr {
				return tc.I
			})
			require.NoError(t, err)
			require.NoError(t, vm.ImportLazy(ctx, s, laz))
			vm.SetEval()
			vm.Run(ctx, 100)
			require.NoError(t, vm.Err())

			wsize := AlignSize(tc.O.Len) / WordBits
			t.Log("bits:", tc.O.Len, "words:", wsize)
			actual := myctests.BP{
				Data: makeBytes(vm.stack[len(vm.stack)-wsize:]),
				Len:  tc.O.Len,
			}
			require.Equal(t, tc.O, actual)
		})
	}
}

func TestAccelerators(t *testing.T) {
	type testCase struct {
		Acc *myc.Lambda
		I   *mycexpr.Expr
		O   []Word
	}
	eb := mycexpr.EB{}
	tcs := []testCase{
		{
			Acc: myccanon.NOT,
			I:   eb.Bit(0),
			O:   []Word{1},
		},
		{
			Acc: myccanon.XOR,
			I:   eb.Product(eb.Bit(1), eb.Bit(0)),
			O:   []Word{1},
		},
		{
			Acc: myccanon.OR,
			I:   eb.Product(eb.Bit(0), eb.Bit(1)),
			O:   []Word{1},
		},
		{
			Acc: myccanon.AND,
			I:   eb.Product(eb.Bit(1), eb.Bit(0)),
			O:   []Word{0},
		},
		{
			Acc: myccanon.B32_Add,
			I:   eb.Product(eb.B32(6), eb.B32(5)),
			O:   []Word{11},
		},
		{
			Acc: myccanon.B32_Sub,
			I:   eb.Product(eb.B32(10), eb.B32(5)),
			O:   []Word{5},
		},
		{
			Acc: myccanon.B32_Mul,
			I:   eb.Product(eb.B32(10), eb.B32(6)),
			O:   []Word{60},
		},
		{
			Acc: myccanon.B32_Div,
			I:   eb.Product(eb.B32(200), eb.B32(50)),
			O:   []Word{4},
		},
		{
			Acc: myccanon.B32_POPCOUNT,
			I:   eb.B32(7),
			O:   []Word{3},
		},
	}
	for i, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx := testutil.Context(t)
			s := testutil.NewStore(t)
			vm := New(0, s, DefaultAccels())
			laz, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb mycexpr.EB) *mycexpr.Expr {
				return eb.Apply(
					eb.Lit(tc.Acc),
					tc.I,
				)
			})
			require.NoError(t, err)
			require.NoError(t, vm.ImportLazy(ctx, s, laz))
			vm.SetEval()
			steps := vm.Run(ctx, 1e9)
			t.Log("steps:", steps)
			require.NoError(t, vm.Err())
			require.Equal(t, tc.O, vm.DumpStack(nil))
			if steps > 15 {
				t.Errorf("too many steps to be accelerated %d", steps)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	s := testutil.NewStore(t)
	vals := myctests.InterestingValues(s)
	vals = append(vals, []myc.Value{
		myccanon.B32_Add,
		myccanon.B32_Sub,
		myccanon.B32_Mul,
		myccanon.B32_Div,
	}...)
	for i, val := range vals {
		t.Run(fmt.Sprintf("%d/%v", i, val), func(t *testing.T) {
			fp1 := myc.Fingerprint(val)

			valWords := encodeWords(t, val)
			tyWords := encodeWords(t, val.Type())
			t2Words := encodeWords(t, val.Type().Type())
			fp2 := fingerprint(Type2(t2Words), tyWords, valWords, int(val.Type().SizeOf()))

			require.Equal(t, cadata.ID(fp1), Ref(fp2).CID())
		})
	}
}

func encodeWords(t testing.TB, x myc.Value) []Word {
	data := myc.MarshalAppend(nil, x)
	words := make([]Word, divCeil(len(data), WordBytes))
	bytesToWords(data, words)
	return words
}
