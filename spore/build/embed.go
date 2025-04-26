package build

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycmem"
)

// Embed embeds a file from p within the context.
func (c *Context) Embed(ctx context.Context, dst cadata.Store, p string) (Value, error) {
	fsx, p, _, err := c.find(p)
	if err != nil {
		return nil, err
	}
	finfo, err := fs.Stat(fsx, p)
	if err != nil {
		return nil, err
	}
	if finfo.IsDir() {
		return c.embedDir(ctx, dst, fsx, p)
	} else {
		return c.embedFile(ctx, dst, fsx, p)
	}
}

func (c *Context) embedDir(ctx context.Context, dst cadata.Store, fsx fs.FS, p string) (Value, error) {
	ents, err := fs.ReadDir(fsx, p)
	if err != nil {
		return nil, err
	}
	var ret []Value
	for _, ent := range ents {
		p2 := filepath.Join(p, ent.Name())
		v, err := c.Embed(ctx, dst, p2)
		if err != nil {
			return nil, err
		}
		ret = append(ret, Product{
			mycmem.NewString(ent.Name()),
			v,
		})
	}
	return Product(ret), nil
}

func (c *Context) embedFile(ctx context.Context, dst cadata.Store, fsx fs.FS, p string) (Value, error) {
	f, err := fsx.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ret []Value
	var buf [mycelium.MaxSizeBytes]byte
	var n int
	emit := func() error {
		byteArr := mycmem.NewByteArray(buf[:n])
		ref, err := mycmem.Post(ctx, dst, byteArr)
		if err != nil {
			return err
		}
		n = 0
		ret = append(ret, mycmem.Product{
			&ref,
			mycmem.NewSize(byteArr.Len()),
		})
		return nil
	}
	for {
		n2, err := io.ReadFull(f, buf[:n])
		n += n2
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if err := emit(); err != nil {
					return nil, err
				}
				break
			}
			return nil, err
		}
		if n == len(buf) {
			if err := emit(); err != nil {
				return nil, err
			}
		}
	}
	if len(ret) == 1 {
		return ret[0], nil
	}
	return mycmem.NewArray(mycmem.StringType(), ret...), nil
}
