package spcmd

import (
	"archive/zip"
	"bytes"

	"go.brendoncarroll.net/star"
	"myceliumweb.org/mycelium/myccmd"
	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/mycss/mycgui"
)

var spRun = star.Command{
	Metadata: star.Metadata{
		Short: "create a new pod to run an executable package",
	},
	Flags: []star.IParam{dbParam, cellParam, netParam, consoleParam},
	Pos:   []star.IParam{pkgParam},
	F: func(c star.Context) error {
		ctx := c.Context
		buf := bytes.Buffer{}
		pkgPath := pkgParam.Load(c)
		if err := buildZipFile(ctx, pkgPath, true, &buf); err != nil {
			return err
		}
		db := dbParam.Load(c)
		sys := mycss.NewSystem(db)
		pod, err := sys.Create(ctx)
		if err != nil {
			return err
		}
		defer sys.Drop(ctx, pod.ID())
		pcfg := myccmd.BuildPodConfig(c)
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			return err
		}
		if err := mycss.ResetZipReader(ctx, pod, zr, pcfg); err != nil {
			return err
		}
		return mycss.Main(ctx, pod)
	},
}

var spRunGui = star.Command{
	Metadata: star.Metadata{
		Short: "run a package with a Graphical User Interface",
	},
	Flags: []star.IParam{dbParam, cellParam, netParam, consoleParam},
	Pos:   []star.IParam{pkgParam},
	F: func(c star.Context) error {
		ctx := c.Context
		buf := bytes.Buffer{}
		pkgPath := pkgParam.Load(c)
		if err := buildZipFile(ctx, pkgPath, false, &buf); err != nil {
			return err
		}
		db := dbParam.Load(c)
		sys := mycss.NewSystem(db)
		pod, err := sys.Create(ctx)
		if err != nil {
			return err
		}
		defer sys.Drop(ctx, pod.ID())
		pcfg := myccmd.BuildPodConfig(c)
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			return err
		}
		if err := mycss.ResetZipReader(ctx, pod, zr, pcfg); err != nil {
			return err
		}
		c.Printf("running GUI...")
		mycgui.Main(ctx, pod)
		return nil
	},
}
