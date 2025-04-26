package spcmd

import (
	"context"
	"io"
	"os"
	"path"
	"strings"

	"go.brendoncarroll.net/star"
	"go.brendoncarroll.net/stdctx/logctx"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/spore/build"
	"myceliumweb.org/mycelium/spore/test"
)

var spBuild = star.Command{
	Metadata: star.Metadata{
		Short: "builds a myczip package and writes it to a file",
	},
	Pos: []star.IParam{outputFileParam, pkgParam},
	F: func(c star.Context) error {
		ctx := c.Context
		pkgPath := pkgParam.Load(c)
		outFile := outputFileParam.Load(c)
		logctx.Infof(ctx, outFile.Name(), pkgPath)
		if err := buildZipFile(ctx, pkgPath, true, outFile); err != nil {
			return err
		}
		return outFile.Close()
	},
}

func buildZipFile(ctx context.Context, pkgPath string, setEntry bool, out io.Writer) error {
	pkgPath = path.Clean(pkgPath)
	pkgPath = strings.TrimPrefix(pkgPath, "./")
	logctx.Infof(ctx, "build %v", pkgPath)
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	bc := build.NewContext([]build.Source{
		{Prefix: "", FS: os.DirFS(dir)},
		build.StdLib(),
	})
	return bc.WriteZip(ctx, pkgPath, setEntry, out)
}

var pkgParam = star.Param[string]{
	Name:  "pkg",
	Parse: star.ParseString,
}

var outputFileParam = star.Param[*os.File]{
	Name:  "o",
	Parse: os.Create,
}

var spTest = star.Command{
	Metadata: star.Metadata{
		Short: "build and run the tests for a package",
	},
	Flags: []star.IParam{},
	Pos:   []star.IParam{pkgParam},
	F: func(c star.Context) error {
		ctx := c.Context
		pkgPath := pkgParam.Load(c)
		pkgPath, shouldList := strings.CutSuffix(pkgPath, "/...")

		bc := build.NewContext([]build.Source{
			{
				Prefix: pkgPath,
				FS:     os.DirFS(pkgPath)},
			build.StdLib(),
		})
		var pkgPaths []string
		if shouldList {
			var err error
			if pkgPaths, err = bc.List(pkgPath); err != nil {
				return err
			}
		} else {
			pkgPaths = []string{pkgPath}
		}

		s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
		for _, pkgPath := range pkgPaths {
			c.Printf("%s", pkgPath)
			pkg, err := bc.Build(ctx, s, pkgPath)
			if err != nil {
				return err
			}
			tests, err := test.List(*pkg)
			if err != nil {
				return err
			}
			if len(tests) == 0 {
				c.Printf(" (no tests)\n")
				continue
			} else {
				c.Printf("\n")
			}
			for _, t := range tests {
				indent := "  "
				c.Printf("%s%s\n", indent, t.Name)
				res, err := test.Run(ctx, s, t)
				if err != nil {
					return err
				}
				if res.Pass {
					c.Printf("   PASS\n")
				} else {
					c.Printf("   FAIL: %v\n", res.Fault)
				}
			}
		}
		return nil
	},
}
