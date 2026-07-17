package mvm2

import "testing"

func TestTwoTimes(t *testing.T) {
    x := 37
    if got := twoTimes(x); got != 2*x {
        t.Fatalf("twoTimes(%d) = %d, want %d", x, got, 2*x)
    }
}
