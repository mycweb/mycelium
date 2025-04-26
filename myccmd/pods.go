package myccmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.brendoncarroll.net/star"
	"myceliumweb.org/mycelium/mycss"
)

var create = star.Command{
	Metadata: star.Metadata{
		Short: "create a pod",
		Tags:  []string{"pod"},
	},
	Flags: []star.IParam{DBParam},
	F: func(c star.Context) error {
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		pod, err := sys.Create(c)
		if err != nil {
			return err
		}
		c.Printf("created pod %d\n", pod.ID())
		return nil
	},
}

var drop = star.Command{
	Metadata: star.Metadata{
		Short: "remove a pod and its data from the system",
		Tags:  []string{"pod"},
	},
	Flags: []star.IParam{DBParam},
	Pos:   []star.IParam{PodIDParam},
	F: func(c star.Context) error {
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		return sys.Drop(c, PodIDParam.Load(c))
	},
}

var list = star.Command{
	Metadata: star.Metadata{
		Short: "list the pods in a system",
		Tags:  []string{"pod"},
	},
	Flags: []star.IParam{DBParam},
	F: func(c star.Context) error {
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		pods, err := sys.List(c)
		if err != nil {
			return err
		}
		c.Printf("ID\n")
		for _, pod := range pods {
			c.Printf("%v\n", pod.ID())
		}
		return nil
	},
}

var reset = star.Command{
	Metadata: star.Metadata{
		Short: "reset",
		Tags:  []string{"pods"},
	},
	Flags: []star.IParam{DBParam, fileParam,
		NetNodeParam, CellParam, ConsoleParam,
	},
	Pos: []star.IParam{PodIDParam},
	F: func(c star.Context) error {
		db := DBParam.Load(c)
		sys := mycss.NewSystem(db)
		pod, err := sys.Get(c, PodIDParam.Load(c))
		if err != nil {
			return err
		}
		f := fileParam.Load(c)
		defer f.Close()
		pcfg := BuildPodConfig(c)
		return mycss.ResetZipFile(c, pod, f, pcfg)
	},
}

var PodIDParam = star.Param[mycss.PodID]{Name: "pod", Parse: ParsePodID}

func ParsePodID(x string) (mycss.PodID, error) {
	n, err := strconv.ParseUint(x, 10, 64)
	return mycss.PodID(n), err
}

var fileParam = star.Param[*os.File]{
	Name: "f",
	Parse: func(x string) (*os.File, error) {
		return os.Open(x)
	},
}

type NetNodeSpec struct {
	Path     string
	KeyIndex uint32
}

var NetNodeParam = star.Param[NetNodeSpec]{
	Name:     "net",
	Repeated: true,
	Parse: func(x string) (NetNodeSpec, error) {
		parts := strings.Split(x, ":")
		if len(parts) < 2 {
			return NetNodeSpec{}, fmt.Errorf("could not parse network node spec from %q", x)
		}
		idx, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return NetNodeSpec{}, err
		}
		return NetNodeSpec{
			Path:     parts[0],
			KeyIndex: uint32(idx),
		}, nil
	},
}

var CellParam = star.Param[string]{
	Name:     "cell",
	Repeated: true,
	Parse:    star.ParseString,
}

var ConsoleParam = star.Param[string]{
	Name:     "console",
	Repeated: true,
	Parse:    star.ParseString,
}

func BuildPodConfig(c star.Context) mycss.PodConfig {
	devs := make(map[string]mycss.DeviceSpec)
	for _, k := range ConsoleParam.LoadAll(c) {
		devs[k] = mycss.DevConsole()
	}
	for _, k := range CellParam.LoadAll(c) {
		devs[k] = mycss.DevCell()
	}
	for _, spec := range NetNodeParam.LoadAll(c) {
		devs[spec.Path] = mycss.DevNetwork(spec.KeyIndex)
	}
	return mycss.PodConfig{
		Devices: devs,
	}
}
