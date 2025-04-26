package mycmem_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myctests"
)

func TestRootEncodeDecode(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	tcs := testValues(s)
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			v := tc
			t.Log(v)
			data, err := myc.SaveRoot(ctx, s, myc.NewAnyValue(v))
			require.NoError(t, err)
			anyval, err := myc.LoadRoot(ctx, s, data)
			require.NoError(t, err)
			actual := anyval.Unwrap()
			if !myc.Equal(v, actual) {
				require.Equal(t, v, actual)
			}
		})
	}
}

func TestCID(t *testing.T) {
	t.Parallel()
	s := testutil.NewStore(t)
	tcs := testValues(s)
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc)
			id := myc.ContentID(tc)
			require.NotEqual(t, cadata.ID{}, id)
		})
	}
}

// TestEqual checks that all the example values are equal to themselves, and not equal to any other value in the list.
// TestEqual scales O(n^2) with the number of test values
func TestEqual(t *testing.T) {
	t.Parallel()
	s := testutil.NewStore(t)
	vals := testValues(s)
	for i := range vals {
		for j := range vals[i:] {
			if i == j {
				assert.True(t, myc.Equal(vals[i], vals[j]), "should be equal %v %v", vals[i], vals[j])
			} else {
				assert.False(t, myc.Equal(vals[i], vals[j]), "should not be equal %v %v", vals[i], vals[j])
			}
		}
	}
}

func TestPostLoad(t *testing.T) {
	t.Parallel()
	s := testutil.NewStore(t)
	tcs := testValues(s)
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := testutil.Context(t)
			v := tc

			ref, err := myc.Post(ctx, s, v)
			require.NoError(t, err)
			actual, err := myc.Load(ctx, s, ref)
			require.NoError(t, err)
			if !myc.Equal(v, actual) {
				require.Equal(t, v, actual)
			}
		})
	}
}

func TestPull(t *testing.T) {
	t.Parallel()
	src := testutil.NewStore(t)
	tcs := testValues(src)
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := testutil.Context(t)
			v := tc
			t.Logf("%v :: %v, %T", v, v.Type(), v)
			dst1 := testutil.NewStore(t)
			err := v.PullInto(ctx, dst1, src)
			require.NoError(t, err)

			data, err := myc.SaveRoot(ctx, dst1, myc.NewAnyValue(v))
			require.NoError(t, err)
			v, err = myc.LoadRoot(ctx, dst1, data)
			require.NoError(t, err)

			dst2 := testutil.NewStore(t)
			err = v.PullInto(ctx, dst2, dst1)
			require.NoError(t, err)
			_, err = myc.LoadRoot(ctx, dst1, data)
			require.NoError(t, err)
		})
	}
}

func TestGolden(t *testing.T) {
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	type testCase struct {
		I myc.Value
		O myctests.BP
	}
	post := func(x myc.Value) *myc.Ref {
		ref, err := myc.Post(ctx, s, x)
		require.NoError(t, err)
		return &ref
	}
	progType0 := myc.NewProgType(0)
	tcs := []testCase{
		// Kinds
		{I: myc.KindKind(), O: myctests.BP_KindKind},
		{I: myc.BitKind(), O: myctests.BP_BitKind},
		{I: myc.ArrayKind(), O: myctests.BP_ArrayKind},
		{I: myc.ProgKind(), O: myctests.BP_ProgKind},
		{I: myc.SumKind(0), O: myctests.BP_SumKind_0},
		{I: myc.ProductKind(0), O: myctests.BP_ProductKind_0},
		{I: myc.ListKind(), O: myctests.BP_ListKind},
		{I: myc.LazyKind(), O: myctests.BP_LazyKind},
		{I: myc.LambdaKind(), O: myctests.BP_LambdaKind},
		{I: myc.FractalKind(), O: myctests.BP_FractalKind},
		{I: myc.PortKind(), O: myctests.BP_PortKind},
		{I: myc.DistinctKind(), O: myctests.BP_DistinctKind},
		{I: myc.AnyTypeKind(), O: myctests.BP_AnyTypeKind},
		{I: myc.AnyValueKind(), O: myctests.BP_AnyValueKind},

		// Bits
		{I: myc.NewBit(0), O: myctests.BP_Bit_0},
		{I: myc.NewBit(1), O: myctests.BP_Bit_1},

		// Refs to Kinds
		{I: post(myc.KindKind()), O: myctests.BP_Ref_KindKind},
		{I: post(myc.BitKind()), O: myctests.BP_Ref_BitKind},
		{I: post(myc.ArrayKind()), O: myctests.BP_Ref_ArrayKind},
		{I: post(myc.ProgKind()), O: myctests.BP_Ref_ProgKind},
		{I: post(myc.SumKind(0)), O: myctests.BP_Ref_SumKind_0},
		{I: post(myc.ProductKind(0)), O: myctests.BP_Ref_ProductKind_0},
		{I: post(myc.ListKind()), O: myctests.BP_Ref_ListKind},
		{I: post(myc.LazyKind()), O: myctests.BP_Ref_LazyKind},
		{I: post(myc.LambdaKind()), O: myctests.BP_Ref_LambdaKind},
		{I: post(myc.FractalKind()), O: myctests.BP_Ref_FractalKind},
		{I: post(myc.PortKind()), O: myctests.BP_Ref_PortKind},
		{I: post(myc.DistinctKind()), O: myctests.BP_Ref_DistinctKind},
		{I: post(myc.AnyTypeKind()), O: myctests.BP_Ref_AnyTypeKind},
		{I: post(myc.AnyValueKind()), O: myctests.BP_Ref_AnyValueKind},

		// Lower Kinded Types
		{I: myc.BitType{}, O: myctests.BP_BitType},
		{I: progType0, O: myctests.BP_ProgType_0}, // ProgType{0}
		{I: myc.SumType{}, O: myctests.BP_SumType_0},
		{I: myc.ProductType{}, O: myctests.BP_ProductType_0},
		{I: myc.AnyTypeType{}, O: myctests.BP_AnyTypeType},
		{I: myc.AnyValueType{}, O: myctests.BP_AnyValueType},

		// Refs to Lower Kinded Types
		{I: post(myc.BitType{}), O: myctests.BP_Ref_BitType},
		{I: post(progType0), O: myctests.BP_Ref_ProgType_0},
		{I: post(myc.SumType{}), O: myctests.BP_Ref_SumType_0},
		{I: post(myc.ProductType{}), O: myctests.BP_Ref_ProductType_0},
		{I: post(myc.AnyTypeType{}), O: myctests.BP_Ref_AnyTypeType},
		{I: post(myc.AnyValueType{}), O: myctests.BP_Ref_AnyValueType},

		// AnyType
		{I: myc.NewAnyType(myc.KindKind()), O: myctests.BP_AnyType_KindKind},
		{I: myc.NewAnyType(myc.BitType{}), O: myctests.BP_AnyType_BitType},
		{I: myc.NewAnyType(progType0), O: myctests.BP_AnyType_ProgType_0},
		{I: myc.NewAnyType(myc.SumType{}), O: myctests.BP_AnyType_SumType_0},
		{I: myc.NewAnyType(myc.ProductType{}), O: myctests.BP_AnyType_ProductType_0},
		{I: myc.NewAnyType(myc.AnyTypeType{}), O: myctests.BP_AnyType_AnyTypeType},
		{I: myc.NewAnyType(myc.AnyValueType{}), O: myctests.BP_AnyType_AnyValueType},

		// Higher Kinded Types
		{I: myc.ArrayOf(myc.BitType{}, 0), O: myctests.BP_ArrayType_Bit_0},
		{I: myc.ArrayOf(myc.BitType{}, 1), O: myctests.BP_ArrayType_Bit_1},
		{I: myc.NewRefType(myc.BitType{}), O: myctests.BP_RefType_Bit},
		{I: myc.SumType{myc.BitType{}}, O: myctests.BP_SumType_Bit},
		{I: myc.ProductType{myc.BitType{}}, O: myctests.BP_ProductType_Bit},
		{I: myc.NewLazyType(myc.BitType{}), O: myctests.BP_LazyType_Bit},
		{I: myc.NewLambdaType(myc.BitType{}, myc.BitType{}), O: myctests.BP_LambdaType_Bit_Bit},
		{I: myc.NewPortType(myc.BitType{}, myc.BitType{}, myc.BitType{}, myc.BitType{}), O: myctests.BP_PortType_Bit_Bit_Bit_Bit},

		// Refs to non-Type Values
		{I: post(post(myc.NewBit(0))), O: myctests.BP_Ref_Ref_Bit_0},
		{I: post(post(myc.NewBit(1))), O: myctests.BP_Ref_Ref_Bit_1},

		// AnyValue
		{I: myc.NewAnyValue(myc.NewBit(0)), O: myctests.BP_AnyValue_Bit_0},
		{I: myc.NewAnyValue(myc.NewBit(1)), O: myctests.BP_AnyValue_Bit_1},
	}
	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d/%v", i, tc.I), func(t *testing.T) {
			actualData := myc.MarshalAppend(nil, tc.I)
			if actualData == nil {
				actualData = []byte{}
			}
			actual := myctests.BP{
				Data: actualData,
				Len:  int(tc.I.Type().SizeOf()),
			}
			require.Equal(t, tc.O, actual)
		})
	}
}

func testValues(s cadata.PostExister) []myc.Value {
	return myctests.InterestingValues(s)
}
