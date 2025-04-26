package myczip

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"sync"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	mycmem "myceliumweb.org/mycelium/mycmem"
)

const rootName = "root"

// Load retrieves the root Value
func Load(zr *zip.Reader) (mycmem.Value, Store, error) {
	zf, err := zr.Open(rootName)
	if err != nil {
		return nil, Store{}, err
	}
	defer zf.Close()
	// TODO: don't use ReadAll
	data, err := io.ReadAll(zf)
	if err != nil {
		return nil, Store{}, err
	}
	s := Store{ZR: zr}
	ctx := context.TODO()
	av, err := mycmem.LoadRoot(ctx, s, data)
	if err != nil {
		return nil, Store{}, err
	}
	return av.Unwrap(), s, nil
}

type File interface {
	io.ReaderAt
	Stat() (fs.FileInfo, error)
}

// LoadFromFile reads a Value from a File
func LoadFromFile(ctx context.Context, f File) (mycmem.Value, cadata.Getter, error) {
	finfo, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	zr, err := zip.NewReader(f, finfo.Size())
	if err != nil {
		return nil, nil, err
	}
	return Load(zr)
}

var (
	_ cadata.Getter  = &Store{}
	_ cadata.Exister = &Store{}
)

type Store struct {
	ZR *zip.Reader
}

func (s Store) Hash(tag *cadata.ID, data []byte) cadata.ID {
	return mycelium.Hash(tag, data)
}

func (s Store) MaxSize() int {
	return mycelium.MaxSizeBytes
}

func (s Store) Get(ctx context.Context, id *cadata.ID, salt *cadata.ID, buf []byte) (int, error) {
	b64ID, err := id.MarshalBase64()
	if err != nil {
		return 0, err
	}
	f, err := s.ZR.Open(string(b64ID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, cadata.ErrNotFound{Key: id}
		}
		return 0, err
	}
	defer f.Close()
	var n int
	for {
		n2, err := f.Read(buf)
		n += n2
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}
	return n, cadata.Check(s.Hash, id, salt, buf[:n])
}

func (s Store) Exists(ctx context.Context, id *cadata.ID) (bool, error) {
	b64ID, err := id.MarshalBase64()
	if err != nil {
		return false, err
	}
	f, err := s.ZR.Open(string(b64ID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	return true, nil
}

// Save adds all of the necessary content to the zip file and then sets the root.
func Save(ctx context.Context, src cadata.Getter, v mycmem.Value, zw *zip.Writer) error {
	dst := newWStore(zw)
	if err := v.PullInto(ctx, dst, src); err != nil {
		return err
	}
	data, err := mycmem.SaveRoot(ctx, dst, mycmem.NewAnyValue(v))
	if err != nil {
		return err
	}
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               rootName,
		UncompressedSize64: uint64(len(data)),
		CompressedSize64:   uint64(len(data)),
	})
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return zw.Flush()
}

// WriteTo creates a zip file wrapping w, calls Save, and then closes the zip file.
func WriteTo(ctx context.Context, src cadata.Getter, v mycmem.Value, w io.Writer) error {
	zw := zip.NewWriter(w)
	if err := Save(ctx, src, v, zw); err != nil {
		return err
	}
	return zw.Close()
}

var _ cadata.PostExister = &zipWStore{}

type zipWStore struct {
	mu sync.Mutex
	zw *zip.Writer
	m  map[cadata.ID]struct{}
}

func newWStore(zw *zip.Writer) *zipWStore {
	return &zipWStore{zw: zw, m: make(map[cadata.ID]struct{})}
}

func (s *zipWStore) Post(ctx context.Context, tag *cadata.ID, data []byte) (cadata.ID, error) {
	id := s.Hash(tag, data)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.m[id]; exists {
		return id, nil
	}
	b64ID, err := id.MarshalBase64()
	if err != nil {
		return cadata.ID{}, err
	}
	var extra []byte
	if tag != nil {
		extra = tag[:]
	}
	w, err := s.zw.CreateRaw(&zip.FileHeader{
		Name:               string(b64ID),
		UncompressedSize64: uint64(len(data)),
		CompressedSize64:   uint64(len(data)),
		Extra:              extra,
	})
	if err != nil {
		return cadata.ID{}, err
	}
	if _, err := w.Write(data); err != nil {
		return cadata.ID{}, err
	}
	s.m[id] = struct{}{}
	return id, nil
}

func (s *zipWStore) Exists(ctx context.Context, id *cadata.ID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.m[*id]
	return exists, nil
}

func (s *zipWStore) Hash(tag *cadata.ID, data []byte) cadata.ID {
	return mycelium.Hash(tag, data)
}

func (s *zipWStore) MaxSize() int {
	return mycelium.MaxSizeBytes
}
