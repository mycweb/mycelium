package main

import (
	"context"
	"testing"

	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/mycss/mycgui"
)

func main() {
	ctx := context.TODO()
	ss := mycss.NewTestSys(new(testing.T))
	pod, err := ss.Create(ctx)
	if err != nil {
		panic(err)
	}
	mycgui.Main(ctx, pod)
}
