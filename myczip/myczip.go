package myczip

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"blobcache.io/blobcache/src/blobcache"
	"myceliumweb.org/mycelium"
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
func LoadFromFile(ctx context.Context, f File) (mycmem.Value, mycelium.RO, error) {
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
	_ mycelium.RO = &Store{}
)

type Store struct {
	ZR *zip.Reader
}

func (s Store) KeyedHash(tag *mycelium.CID, data []byte) mycelium.CID {
	return mycelium.Hash(tag, data)
}

func (s Store) MaxSize() int {
	return mycelium.MaxSizeBytes
}

func (s Store) Get(ctx context.Context, id mycelium.CID, salt *mycelium.CID, buf []byte) (int, error) {
	b64ID, err := id.MarshalBase64()
	if err != nil {
		return 0, err
	}
	f, err := s.ZR.Open(string(b64ID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, blobcache.ErrNotFound{CID: id}
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
	if have := s.KeyedHash(salt, buf[:n]); have != id {
		return 0, fmt.Errorf("myczip: blob hash mismatch HAVE: %v WANT: %v", have, id)
	}
	return n, nil
}

func (s Store) Exists(ctx context.Context, ids []mycelium.CID, bm *blobcache.BitMap) error {
	for i, id := range ids {
		b64ID, err := id.MarshalBase64()
		if err != nil {
			return err
		}
		f, err := s.ZR.Open(string(b64ID))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		_ = f.Close()
		bm.Set(i)
	}
	return nil
}

// Save adds all of the necessary content to the zip file and then sets the root.
func Save(ctx context.Context, src mycelium.RO, v mycmem.Value, zw *zip.Writer) error {
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
func WriteTo(ctx context.Context, src mycelium.RO, v mycmem.Value, w io.Writer) error {
	zw := zip.NewWriter(w)
	if err := Save(ctx, src, v, zw); err != nil {
		return err
	}
	return zw.Close()
}

var _ mycelium.WO = &zipWStore{}

type zipWStore struct {
	mu sync.Mutex
	zw *zip.Writer
	m  map[mycelium.CID]struct{}
}

func newWStore(zw *zip.Writer) *zipWStore {
	return &zipWStore{zw: zw, m: make(map[mycelium.CID]struct{})}
}

func (s *zipWStore) Post(ctx context.Context, tag *mycelium.CID, data []byte) (mycelium.CID, error) {
	id := s.KeyedHash(tag, data)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.m[id]; exists {
		return id, nil
	}
	b64ID, err := id.MarshalBase64()
	if err != nil {
		return mycelium.CID{}, err
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
		return mycelium.CID{}, err
	}
	if _, err := w.Write(data); err != nil {
		return mycelium.CID{}, err
	}
	s.m[id] = struct{}{}
	return id, nil
}

func (s *zipWStore) Exists(ctx context.Context, ids []mycelium.CID, bm *blobcache.BitMap) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, id := range ids {
		if _, exists := s.m[id]; exists {
			bm.Set(i)
		}
	}
	return nil
}

func (s *zipWStore) KeyedHash(tag *mycelium.CID, data []byte) mycelium.CID {
	return mycelium.Hash(tag, data)
}

func (s *zipWStore) MaxSize() int {
	return mycelium.MaxSizeBytes
}
