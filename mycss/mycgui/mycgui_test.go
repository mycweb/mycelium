package mycgui

import (
	"fmt"
	"reflect"
	"testing"

	myc "myceliumweb.org/mycelium/mycmem"

	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshal(t *testing.T) {
	tcs := []any{
		FillOp{
			Color: Color{1, 2, 3, 4},
		},
		Op{
			Fill: &FillOp{
				Color: Color{1, 2, 3, 4},
			},
		},
	}
	for i, x := range tcs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			mval := myc.ConvertTo(x)
			dst := reflect.New(reflect.TypeOf(x)).Interface()
			require.NoError(t, myc.ConvertFrom(mval, dst))
			actual := reflect.ValueOf(dst).Elem().Interface()
			require.Equal(t, x, actual)
		})
	}
}
