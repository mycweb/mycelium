package mycmem

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypeSupersets(t *testing.T) {
	t.Parallel()
	type testCase struct {
		A, B Type
	}

	// Does A superset B?
	yess := []testCase{}
	for i, tc := range yess {
		t.Run("Yes/"+strconv.Itoa(i), func(t *testing.T) {
			actual := Supersets(tc.A, tc.B)
			require.True(t, actual)
		})
	}

	nos := []testCase{
		{
			A: ListOf(B32Type()),
			B: ListOf(B64Type()),
		},
		{
			A: ListOf(B32Type()),
			B: ArrayOf(B32Type(), 100),
		},
	}
	for i, tc := range nos {
		t.Run("No/"+strconv.Itoa(i), func(t *testing.T) {
			actual := Supersets(tc.A, tc.B)
			require.False(t, actual)
		})
	}
}

func TestTypeContains(t *testing.T) {
	t.Parallel()
	// Does Type contain Value?
	tcs := []struct {
		T   Type
		V   Value
		Yes bool
	}{
		{ProductType{}, Product{}, true},
		{B32Type(), NewB32(0), true},

		{ArrayOf(B32Type(), 1), NewArray(B32Type(), NewB32(123)), true},
		{ListOf(B32Type()), NewArray(B32Type(), NewB32(123)), false},
	}
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual := TypeContains(tc.T, tc.V)
			require.Equal(t, tc.Yes, actual)
		})
	}
}

func TestTypeOf(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		V Value
		T Type
	}{
		{V: NewString("abc"), T: ListOf(ByteType())},
		{V: NewB32(123), T: B32Type()},

		{V: Product{NewB32(1)}, T: ProductType{B32Type()}},
		{V: NewArray(B32Type(), NewB32(1), NewB32(2), NewB32(3)), T: ArrayOf(B32Type(), 3)},

		{V: ProductType{}, T: ProductKind(0)},
		{V: ProductType{B32Type()}, T: ProductKind(1)},
		{V: ArrayOf(B32Type(), 100), T: ArrayKind()},
		{V: ProductKind(10), T: KindKind()},
		{V: ArrayKind(), T: KindKind()},
	}
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual := tc.V.Type()
			require.Equal(t, tc.T, actual)
		})
	}
}
