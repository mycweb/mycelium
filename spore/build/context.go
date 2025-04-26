package build

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myczip"
	"myceliumweb.org/mycelium/spore"
	"myceliumweb.org/mycelium/spore/compile"

	"go.brendoncarroll.net/exp/slices2"
	"go.brendoncarroll.net/stdctx/logctx"
)

type (
	Namespace = myccanon.Namespace
	Value     = mycmem.Value
	Product   = mycmem.Product
)

const (
	FilepathExtension = ".sp"
)

// Context is a build context, it has a list of sources,
// and an import cache for packages.
type Context struct {
	sources []Source

	cache map[string]compile.Package
}

// Source is a mapping from a Prefix to an FS
type Source struct {
	Prefix string
	FS     fs.FS
	Base   map[string]Namespace
}

func NewContext(sources []Source) *Context {
	return &Context{
		sources: sources,

		cache: make(map[string]compile.Package),
	}
}

// List produces a list of package names with the prefix
func (c *Context) List(prefix string) ([]string, error) {
	fsx, relPath, _, err := c.find(prefix)
	if err != nil {
		return nil, err
	}
	var ret []string
	if err := fs.WalkDir(fsx, relPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		ents, err := fs.ReadDir(fsx, path)
		if err != nil {
			return err
		}
		if containsSporeFiles(ents) {
			ret = append(ret, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

// find returns the first fs.FS and a path that has a directory entry for the package
func (c *Context) find(x string) (fs.FS, string, Namespace, error) {
	for _, src := range c.sources {
		if p, found := strings.CutPrefix(x, src.Prefix); found {
			if p == "" {
				p = "."
			}
			finfo, err := fs.Stat(src.FS, p)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, "", nil, err
			}
			if !finfo.IsDir() {
				return nil, p, nil, fmt.Errorf("file at %q is not a directory", p)
			}
			return src.FS, p, src.Base[p], nil
		}
	}
	return nil, "", nil, errPackageNotFound(x)
}

// WriteZip takes the package at name in this context and writes it out to w.
func (c *Context) WriteZip(ctx context.Context, pkgName string, isExec bool, w io.Writer) error {
	s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	pkg, err := c.Build(ctx, s, pkgName)
	if err != nil {
		return err
	}
	if len(pkg.NS) == 0 {
		logctx.Warnf(ctx, "package %q has no published symbols", pkgName)
	}
	if isExec {
		val, exists := pkg.NS[EntrypointName]
		if !exists {
			return fmt.Errorf("package does not have an entry for %v, cannot set entrypoint", EntrypointName)
		}
		lam, ok := val.(*mycmem.Lambda)
		if !ok {
			return fmt.Errorf("main is not a vaid entrypoint")
		}
		lam, err := makeEntrypoint(lam)
		if err != nil {
			return err
		}
		pkg.NS[""] = lam
	}
	mval := pkg.NS.ToMycelium()
	return myczip.WriteTo(ctx, s, mval, w)
}

// compile invokes the compiler on the files in a source directory
func (c *Context) compile(ctx context.Context, _ cadata.PostExister, base Namespace, sd *SourceDir) (*compile.Package, error) {
	s2 := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	comp := compile.New(s2, spore.Preamble())
	pkg, err := comp.Compile(ctx, base, c.cache, slices2.Map(sd.Files, func(x *SourceFile) compile.SourceFile {
		return x.SourceFile
	}))
	if err != nil {
		return nil, fmt.Errorf("[compile %q] %w", sd.Path, err)
	}
	return pkg, nil
}

func LoadPkg(zr *zip.Reader) (*compile.Package, cadata.Getter, error) {
	val, src, err := myczip.Load(zr)
	if err != nil {
		return nil, nil, err
	}
	ns := myccanon.Namespace{}
	if err := ns.FromMycelium(val); err != nil {
		return nil, nil, err
	}
	return &compile.Package{
		NS: ns,
	}, src, nil
}

const EntrypointName = "main"

func makeEntrypoint(lam *mycmem.Lambda) (*mycmem.Lambda, error) {
	inType := lam.LambdaType().In()
	outType := lam.LambdaType().Out()
	if pty, ok := inType.(mycmem.ProductType); ok && len(pty) == 1 {
		return mycexpr.BuildLambda(pty[0], outType, func(eb mycexpr.EB) *mycexpr.Expr {
			return eb.Apply(eb.Lit(lam), eb.Product(eb.P(0)))
		})
	} else {
		return nil, fmt.Errorf("cannot make lambda of type %v entrypoint", lam.LambdaType())
	}
}

func containsSporeFiles(ents []fs.DirEntry) bool {
	for _, ent := range ents {
		if strings.HasSuffix(ent.Name(), FilepathExtension) {
			return true
		}
	}
	return false
}
