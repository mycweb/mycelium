package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spore/compile"
	"myceliumweb.org/mycelium/spore/parser"
	"myceliumweb.org/mycelium/spore/stdlib"
)

type Package = compile.Package

func (c *Context) Build(ctx context.Context, dst cadata.PostExister, name string) (*Package, error) {
	if pkg, exists := c.cache[name]; exists {
		return &pkg, nil
	}
	pkg, err := func() (*Package, error) {
		fsx, p, base, err := c.find(name)
		if err != nil {
			return nil, err
		}
		sd, err := c.buildDir(fsx, p)
		if err != nil {
			return nil, err
		}
		for _, sf := range sd.Files {
			for _, istmt := range sf.DirectDeps {
				if _, exists := c.cache[istmt.Target]; !exists {
					if _, err := c.Build(ctx, dst, istmt.Target); err != nil {
						return nil, err
					}
				}
			}
		}
		return c.compile(ctx, dst, base, sd)
	}()
	if err != nil {
		return nil, err
	}
	if pkg == nil {
		return nil, errPackageNotFound(name)
	}
	c.cache[name] = *pkg
	return pkg, nil
}

type SourceDir struct {
	// Path is the filepath to the directory
	Path string
	// Files are the files in the directory that were considered part of the package
	Files []*SourceFile
}

// buildDir imports a directory at path p
func (c *Context) buildDir(fsx fs.FS, p string) (*SourceDir, error) {
	ents, err := fs.ReadDir(fsx, p)
	if err != nil {
		pe := new(fs.PathError)
		if errors.As(err, &pe) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	sd := SourceDir{
		Path: p,
	}
	for _, ent := range ents {
		if filepath.Ext(ent.Name()) != FilepathExtension {
			continue
		}
		p2 := path.Join(p, ent.Name())
		sf, err := c.buildFile(fsx, p2)
		if err != nil {
			return nil, err
		}
		sd.Files = append(sd.Files, sf)
	}
	return &sd, nil
}

type SourceFile struct {
	compile.SourceFile
	DirectDeps []compile.ImportStmt
}

func (c *Context) buildFile(fsx fs.FS, p string) (*SourceFile, error) {
	f, err := fsx.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	par := parser.NewParser(bytes.NewReader(src))
	span, contents, err := parser.ReadAll(par)
	if err != nil {
		return nil, fmt.Errorf("parsing file %q: %w", p, err)
	}
	sf := compile.SourceFile{
		Filename: path.Base(p),
		Source:   src,
		Nodes:    contents,
		Span:     span,
		Newlines: indexNewlines(src),
	}
	istmts, _, err := sf.ScanImports(0)
	if err != nil {
		return nil, err
	}
	return &SourceFile{
		SourceFile: sf,
		DirectDeps: istmts,
	}, nil
}

func errPackageNotFound(name string) error {
	return fmt.Errorf("package %q not found", name)
}

func StdLib() Source {
	fs.WalkDir(stdlib.FS, "", func(path string, d fs.DirEntry, err error) error {
		return nil
	})
	return Source{"", stdlib.FS, stdlib.Base}
}

func indexNewlines(src []byte) (ret []uint32) {
	for i, b := range src {
		if b == '\n' {
			ret = append(ret, uint32(i))
		}
	}
	return ret
}
