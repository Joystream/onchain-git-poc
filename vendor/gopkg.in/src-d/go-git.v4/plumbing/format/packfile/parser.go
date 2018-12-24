package packfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

var (
	// ErrReferenceDeltaNotFound is returned when the reference delta is not
	// found.
	ErrReferenceDeltaNotFound = errors.New("reference delta not found")

	// ErrNotSeekableSource is returned when the source for the parser is not
	// seekable and a storage was not provided, so it can't be parsed.
	ErrNotSeekableSource = errors.New("parser source is not seekable and storage was not provided")

	// ErrDeltaNotCached is returned when the delta could not be found in cache.
	ErrDeltaNotCached = errors.New("delta could not be found in cache")
)

// Observer interface is implemented by index encoders.
type Observer interface {
	// OnHeader is called when a new packfile is opened.
	OnHeader(count uint32) error
	// OnInflatedObjectHeader is called for each object header read.
	OnInflatedObjectHeader(t plumbing.ObjectType, objSize int64, pos int64) error
	// OnInflatedObjectContent is called for each decoded object.
	OnInflatedObjectContent(h plumbing.Hash, pos int64, crc uint32, content []byte) error
	// OnFooter is called when decoding is done.
	OnFooter(h plumbing.Hash) error
}

// Parser decodes a packfile and calls any observer associated to it. Is used
// to generate indexes.
type Parser struct {
	storage    storer.EncodedObjectStorer
	scanner    *Scanner
	count      uint32
	oi         []*objectInfo
	oiByHash   map[plumbing.Hash]*objectInfo
	oiByOffset map[int64]*objectInfo
	hashOffset map[plumbing.Hash]int64
	checksum   plumbing.Hash

	cache *cache.BufferLRU
	// delta content by offset, only used if source is not seekable
	deltas map[int64][]byte

	ob []Observer
}

// NewParser creates a new Parser. The Scanner source must be seekable.
// If it's not, NewParserWithStorage should be used instead.
func NewParser(scanner *Scanner, ob ...Observer) (*Parser, error) {
	return NewParserWithStorage(scanner, nil, ob...)
}

// NewParserWithStorage creates a new Parser. The scanner source must either
// be seekable or a storage must be provided.
func NewParserWithStorage(
	scanner *Scanner,
	storage storer.EncodedObjectStorer,
	ob ...Observer,
) (*Parser, error) {
	if !scanner.IsSeekable && storage == nil {
		return nil, ErrNotSeekableSource
	}

	var deltas map[int64][]byte
	if !scanner.IsSeekable {
		deltas = make(map[int64][]byte)
	}

	return &Parser{
		storage: storage,
		scanner: scanner,
		ob:      ob,
		count:   0,
		cache:   cache.NewBufferLRUDefault(),
		deltas:  deltas,
	}, nil
}

func (p *Parser) forEachObserver(f func(o Observer) error) error {
	for _, o := range p.ob {
		if err := f(o); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) onHeader(count uint32) error {
	return p.forEachObserver(func(o Observer) error {
		// fmt.Fprintf(os.Stderr, "Calling onHeader of observer %v\n", o)
		return o.OnHeader(count)
	})
}

func (p *Parser) onInflatedObjectHeader(
	t plumbing.ObjectType,
	objSize int64,
	pos int64,
) error {
	return p.forEachObserver(func(o Observer) error {
		return o.OnInflatedObjectHeader(t, objSize, pos)
	})
}

func (p *Parser) onInflatedObjectContent(
	h plumbing.Hash,
	pos int64,
	crc uint32,
	content []byte,
) error {
	return p.forEachObserver(func(o Observer) error {
		return o.OnInflatedObjectContent(h, pos, crc, content)
	})
}

func (p *Parser) onFooter(h plumbing.Hash) error {
	return p.forEachObserver(func(o Observer) error {
		return o.OnFooter(h)
	})
}

// Parse starts the decoding phase of the packfile.
func (p *Parser) Parse() (plumbing.Hash, error) {
	fmt.Fprintf(os.Stderr, "Commencing parsing\n")
	if err := p.init(); err != nil {
		return plumbing.ZeroHash, err
	}

	if err := p.indexObjects(); err != nil {
		return plumbing.ZeroHash, err
	}

	var err error
	p.checksum, err = p.scanner.Checksum()
	if err != nil && err != io.EOF {
		return plumbing.ZeroHash, err
	}

	fmt.Fprintf(os.Stderr, "Parser - The packfile checksum is %s\n", p.checksum)
	if err := p.resolveDeltas(); err != nil {
		return plumbing.ZeroHash, err
	}

	if err := p.onFooter(p.checksum); err != nil {
		return plumbing.ZeroHash, err
	}

	return p.checksum, nil
}

func (p *Parser) init() error {
	_, c, err := p.scanner.Header()
	if err != nil {
		return err
	}

	if err := p.onHeader(c); err != nil {
		return err
	}

	p.count = c
	p.oiByHash = make(map[plumbing.Hash]*objectInfo, p.count)
	p.oiByOffset = make(map[int64]*objectInfo, p.count)
	p.oi = make([]*objectInfo, p.count)

	return nil
}

func (p *Parser) indexObjects() error {
	fmt.Printf("Parser indexing %d objects\n", p.count)
	buf := new(bytes.Buffer)

	for i := uint32(0); i < p.count; i++ {
		buf.Reset()

		oh, err := p.scanner.NextObjectHeader()
		if err != nil {
			return err
		}

		delta := false
		var ota *objectInfo
		switch t := oh.Type; t {
		case plumbing.OFSDeltaObject:
			fmt.Fprintf(os.Stderr, "Parser encountered OFSDeltaObject\n")
			delta = true

			parent, ok := p.oiByOffset[oh.OffsetReference]
			if !ok {
				return plumbing.ErrObjectNotFound
			}

			fmt.Fprintf(os.Stderr, "Parser - Parent object hash: %s\n", parent.SHA1)
			ota = newDeltaObject(oh.Offset, oh.Length, t, parent)
			fmt.Fprintf(os.Stderr, "Parser appending delta object to parent's children\n")
			parent.Children = append(parent.Children, ota)
		case plumbing.REFDeltaObject:
			fmt.Fprintf(os.Stderr, "Parser encountered REFDeltaObject\n")
			delta = true
			parent, ok := p.oiByHash[oh.Reference]
			if !ok {
				fmt.Fprintf(os.Stderr, "Parser - Parent hash: %s, external reference\n", parent.SHA1)
				// can't find referenced object in this pack file
				// this must be a "thin" pack.
				parent = &objectInfo{ //Placeholder parent
					SHA1:        oh.Reference,
					ExternalRef: true, // mark as an external reference that must be resolved
					Type:        plumbing.AnyObject,
					DiskType:    plumbing.AnyObject,
				}
				p.oiByHash[oh.Reference] = parent
			} else {
				fmt.Fprintf(os.Stderr, "Parser - Parent hash: %s\n", parent.SHA1)
			}

			ota = newDeltaObject(oh.Offset, oh.Length, t, parent)
			fmt.Fprintf(os.Stderr, "Parser appending delta object to parent's children\n")
			parent.Children = append(parent.Children, ota)

		default:
			fmt.Fprintf(os.Stderr, "Parser encountered base object\n")
			ota = newBaseObject(oh.Offset, oh.Length, t)
		}

		_, crc, err := p.scanner.NextObject(buf)
		if err != nil {
			return err
		}

		ota.Crc32 = crc
		ota.Length = oh.Length

		fmt.Fprintf(os.Stderr, "Parser finished getting object bytes\n")
		data := buf.Bytes()
		if !delta {
			sha1, err := getSHA1(ota.Type, data)
			fmt.Fprintf(os.Stderr, "Parser - This is not a delta object, its hash is %s\n", sha1)
			if err != nil {
				return err
			}

			ota.SHA1 = sha1
			p.oiByHash[ota.SHA1] = ota
		}

		if p.storage != nil && !delta {
			fmt.Fprintf(os.Stderr, "Parser writing object of type %s to storage: %s\n", ota.Type, ota.SHA1)
			obj := new(plumbing.MemoryObject)
			obj.SetSize(oh.Length)
			obj.SetType(oh.Type)
			if _, err := obj.Write(data); err != nil {
				fmt.Fprintf(os.Stderr, "Parser writing data to memory object failed: %s\n", err)
				return err
			}

			fmt.Fprintf(os.Stderr, "Parser writing object to storage\n")
			if _, err := p.storage.SetEncodedObject(obj); err != nil {
				fmt.Fprintf(os.Stderr, "Parser writing object to storage failed: %s\n", err)
				return err
			}
		} else if delta {
			fmt.Fprintf(os.Stderr, "Parser not writing object to storage because it's a delta\n")
		} else {
			fmt.Fprintf(os.Stderr, "Parser not writing object to storage because there is no storage\n")
		}

		if delta && !p.scanner.IsSeekable {
			fmt.Fprintf(os.Stderr, "Parser - scanner isn't seekable, copying data into p.deltas[%d]\n",
				oh.Offset)
			p.deltas[oh.Offset] = make([]byte, len(data))
			copy(p.deltas[oh.Offset], data)
		}

		fmt.Fprintf(os.Stderr, "Parser registering object info at p.oiByOffset[%d] and p.oi[%d]\n",
			oh.Offset, i)
		p.oiByOffset[oh.Offset] = ota
		p.oi[i] = ota
	}

	fmt.Fprintf(os.Stderr, "Parser finished indexing objects\n")
	return nil
}

func (p *Parser) resolveDeltas() error {
	fmt.Fprintf(os.Stderr, "Parser resolving deltas of %d objects\n", len(p.oi))
	for _, obj := range p.oi {
		fmt.Fprintf(os.Stderr, "Parser getting content of object %s (disk type: %s, type: %s)\n",
			obj.SHA1, obj.DiskType, obj.Type)
		content, err := p.get(obj)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parser failed to get content of object %s\n", obj.SHA1)
			return err
		}

		if err := p.onInflatedObjectHeader(obj.Type, obj.Length, obj.Offset); err != nil {
			return err
		}

		if err := p.onInflatedObjectContent(obj.SHA1, obj.Offset, obj.Crc32, content); err != nil {
			return err
		}

		if !obj.IsDelta() && len(obj.Children) > 0 {
			fmt.Fprintf(os.Stderr, "Parser handling %d children of non-delta object\n", len(obj.Children))
			for _, child := range obj.Children {
				if _, err := p.resolveObject(child, content); err != nil {
					return err
				}
			}

			// Remove the delta from the cache.
			if obj.DiskType.IsDelta() && !p.scanner.IsSeekable {
				fmt.Fprintf(os.Stderr, "Parser removing object from delta cache\n")
				delete(p.deltas, obj.Offset)
			}
		}
	}

	return nil
}

func (p *Parser) get(o *objectInfo) (b []byte, err error) {
	var ok bool
	if !o.ExternalRef { // skip cache check for placeholder parents
		b, ok = p.cache.Get(o.Offset)
	}

	// If it's not on the cache and is not a delta we can try to find it in the
	// storage, if there's one. External refs must enter here.
	if !ok && p.storage != nil && !o.Type.IsDelta() {
		e, err := p.storage.EncodedObject(plumbing.AnyObject, o.SHA1)
		if err != nil {
			return nil, err
		}
		o.Type = e.Type()

		r, err := e.Reader()
		if err != nil {
			return nil, err
		}

		b = make([]byte, e.Size())
		if _, err = r.Read(b); err != nil {
			return nil, err
		}
	}

	if b != nil {
		return b, nil
	}

	if o.ExternalRef {
		// we were not able to resolve a ref in a thin pack
		fmt.Fprintf(os.Stderr, "Parser unable to resolve reference delta of object '%s'\n", o.SHA1)
		debug.PrintStack()
		return nil, ErrReferenceDeltaNotFound
	}

	var data []byte
	if o.DiskType.IsDelta() {
		fmt.Fprintf(os.Stderr, "Parser getting parent of object %s: %s\n", o.SHA1, o.Parent.SHA1)
		base, err := p.get(o.Parent)
		if err != nil {
			return nil, err
		}

		data, err = p.resolveObject(o, base)
		if err != nil {
			return nil, err
		}
	} else {
		data, err = p.readData(o)
		if err != nil {
			return nil, err
		}
	}

	if len(o.Children) > 0 {
		p.cache.Put(o.Offset, data)
	}

	return data, nil
}

func (p *Parser) resolveObject(
	o *objectInfo,
	base []byte,
) ([]byte, error) {
	if !o.DiskType.IsDelta() {
		return nil, nil
	}

	fmt.Fprintf(os.Stderr, "Parser resolving delta object with hash %s\n", o.SHA1)
	data, err := p.readData(o)
	if err != nil {
		return nil, err
	}

	data, err = applyPatchBase(o, data, base)
	if err != nil {
		return nil, err
	}

	if p.storage != nil {
		obj := new(plumbing.MemoryObject)
		obj.SetSize(o.Size())
		obj.SetType(o.Type)
		if _, err := obj.Write(data); err != nil {
			return nil, err
		}

		fmt.Fprintf(os.Stderr, "Parser setting patched object in storage; hash: %s, type: %s\n", obj.Hash(),
			obj.Type())
		if _, err := p.storage.SetEncodedObject(obj); err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (p *Parser) readData(o *objectInfo) ([]byte, error) {
	if !p.scanner.IsSeekable && o.DiskType.IsDelta() {
		data, ok := p.deltas[o.Offset]
		if !ok {
			return nil, ErrDeltaNotCached
		}

		return data, nil
	}

	if _, err := p.scanner.SeekFromStart(o.Offset); err != nil {
		return nil, err
	}

	if _, err := p.scanner.NextObjectHeader(); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, _, err := p.scanner.NextObject(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func applyPatchBase(ota *objectInfo, data, base []byte) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Parser applying patch to base for delta object\n")
	patched, err := PatchDelta(base, data)
	if err != nil {
		return nil, err
	}

	if ota.SHA1 == plumbing.ZeroHash {
		ota.Type = ota.Parent.Type
		sha1, err := getSHA1(ota.Type, patched)
		if err != nil {
			return nil, err
		}

		ota.SHA1 = sha1
		ota.Length = int64(len(patched))
	}

	return patched, nil
}

func getSHA1(t plumbing.ObjectType, data []byte) (plumbing.Hash, error) {
	hasher := plumbing.NewHasher(t, int64(len(data)))
	if _, err := hasher.Write(data); err != nil {
		return plumbing.ZeroHash, err
	}

	return hasher.Sum(), nil
}

type objectInfo struct {
	Offset      int64
	Length      int64
	Type        plumbing.ObjectType
	DiskType    plumbing.ObjectType
	ExternalRef bool // indicates this is an external reference in a thin pack file

	Crc32 uint32

	Parent   *objectInfo
	Children []*objectInfo
	SHA1     plumbing.Hash
}

func newBaseObject(offset, length int64, t plumbing.ObjectType) *objectInfo {
	return newDeltaObject(offset, length, t, nil)
}

func newDeltaObject(
	offset, length int64,
	t plumbing.ObjectType,
	parent *objectInfo,
) *objectInfo {
	obj := &objectInfo{
		Offset:   offset,
		Length:   length,
		Type:     t,
		DiskType: t,
		Crc32:    0,
		Parent:   parent,
	}

	return obj
}

func (o *objectInfo) IsDelta() bool {
	return o.Type.IsDelta()
}

func (o *objectInfo) Size() int64 {
	return o.Length
}
