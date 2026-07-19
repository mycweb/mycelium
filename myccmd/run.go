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
	Flags: map[string]star.Flag{
		"db":      &DBParam,
		"f":       &fileParam,
		"net":     &NetNodeParam,
		"cell":    &CellParam,
		"console": &ConsoleParam,
	},
	F: func(c star.Context) error {
		db, err := loadDB(c)
		if err != nil {
			return err
		}
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
	Pos:   []star.Positional{&podIDsParam},
	Flags: map[string]star.Flag{"db": &DBParam},
	F: func(c star.Context) error {
		// setup system
		db, err := loadDB(c)
		if err != nil {
			return err
		}
		sys := mycss.NewSystem(db)

		var pods []*mycss.Pod
		for _, pid := range podIDsParam.Load(c) {
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
	Flags: map[string]star.Flag{"db": &DBParam, "l": &ListenerParam},
	F: func(c star.Context) error {
		// setup system
		db, err := loadDB(c)
		if err != nil {
			return err
		}
		sys := mycss.NewSystem(db)
		// setup listener
		lis, err := loadListener(c)
		if err != nil {
			return err
		}

		eg, ctx := errgroup.WithContext(c.Context)
		eg.Go(func() error { return sys.Run(ctx) })
		eg.Go(func() error { return mychui.Serve(ctx, lis, sys) })
		return eg.Wait()
	},
}

var podIDsParam = star.Repeated[mycss.PodID]{PosName: "pid", Parse: ParsePodID}
