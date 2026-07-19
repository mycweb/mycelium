package myccmd

import (
	"encoding/json"
	"fmt"
	"io"

	"go.brendoncarroll.net/star"
	"myceliumweb.org/mycelium/mycss"
)

var invokeJSON = star.Command{
	Metadata: star.Metadata{
		Short: "call invokeJSON method in a pod with JSON data",
	},
	Flags: map[string]star.Flag{"db": &DBParam, "pod": &PodIDParam},
	F: func(c star.Context) error {
		db, err := loadDB(c)
		if err != nil {
			return err
		}
		sys := mycss.NewSystem(db)
		ctx := c.Context
		sys.Get(ctx, PodIDParam.Load(c))
		pod, err := sys.Create(ctx)
		if err != nil {
			return err
		}
		input, err := io.ReadAll(c.StdIn)
		if err != nil {
			return err
		}
		if !json.Valid(input) {
			return fmt.Errorf("invalid JSON: %s", input)
		}
		out, err := mycss.InvokeJSON(ctx, pod, input)
		if err != nil {
			return err
		}
		_, err = c.StdOut.Write(out)
		return err
	},
}
