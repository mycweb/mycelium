package myccmd

import (
	"go.brendoncarroll.net/star"
	"golang.org/x/sync/errgroup"

	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/mycss/mychui"
)

var run = star.Command{
	Metadata: star.Metadata{
		Short: "run an executable namespace in a new pod",
	},
	Flags: []star.IParam{DBParam, fileParam,
		NetNodeParam, CellParam, ConsoleParam,
	},
	F: func(c star.Context) error {
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		ctx := c.Context
		pod, err := sys.Create(ctx)
		if err != nil {
			return err
		}
		f := fileParam.Load(c)
		pcfg := BuildPodConfig(c)
		if err := mycss.ResetZipFile(ctx, pod, f, pcfg); err != nil {
			return err
		}
		return mycss.Main(ctx, pod)
	},
}

var runPods = star.Command{
	Metadata: star.Metadata{
		Short: "run existing pods",
	},
	Pos:   []star.IParam{podIDsParam},
	Flags: []star.IParam{DBParam},
	F: func(c star.Context) error {
		// setup system
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)

		var pods []*mycss.Pod
		for _, pid := range podIDsParam.LoadAll(c) {
			pod, err := sys.Get(c.Context, pid)
			if err != nil {
				return err
			}
			pods = append(pods, pod)
		}
		eg, ctx := errgroup.WithContext(c.Context)
		for _, pod := range pods {
			pod := pod
			eg.Go(func() error {
				return mycss.Main(ctx, pod)
			})
		}
		return eg.Wait()
	},
}

var serve = star.Command{
	Metadata: star.Metadata{
		Short: "run all the pods in this system, and serve the HTTP UI",
	},
	Flags: []star.IParam{DBParam, ListenerParam},
	F: func(c star.Context) error {
		// setup system
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		// setup listener
		lis := ListenerParam.Load(c)

		eg, ctx := errgroup.WithContext(c.Context)
		eg.Go(func() error { return sys.Run(ctx) })
		eg.Go(func() error { return mychui.Serve(ctx, lis, sys) })
		return eg.Wait()
	},
}

var podIDsParam = star.Param[mycss.PodID]{
	Name:     "pid",
	Repeated: true,
	Parse:    ParsePodID,
}
