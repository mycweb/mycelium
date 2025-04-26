package myccmd

import (
	"context"
	"encoding/hex"

	"go.brendoncarroll.net/star"
	mycelium "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myczip"
)

var zipCmd = star.NewDir(star.Metadata{
	Short: "deal with myczip files",
},
	map[star.Symbol]star.Command{
		"inspect": zipInspectCmd,
	},
)

var zipInspectCmd = star.Command{
	Pos: []star.IParam{fileParam},
	F: func(c star.Context) error {
		f := fileParam.Load(c)
		finfo, err := f.Stat()
		if err != nil {
			return err
		}
		c.Printf("FILE-SIZE: %d bytes\n", finfo.Size())
		ctx := context.Background()
		root, _, err := myczip.LoadFromFile(ctx, f)
		if err != nil {
			return err
		}
		c.Printf("ROOT-TYPE: %v\n", root.Type())
		c.Printf("ROOT: %v\n", mycelium.Pretty(root))
		data := mycelium.MarshalAppend(nil, root)
		c.Printf("ROOT-HEX:\n%s", hex.Dump(data))
		c.Printf("\n")
		return nil
	},
}
