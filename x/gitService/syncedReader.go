package gitService

import (
	"io"
	"sync/atomic"
)

type syncedReader struct {
	w io.Writer
	r io.ReadSeeker

	blocked, done uint32
	written, read uint64
	news          chan bool
}

func newSyncedReader(w io.Writer, r io.ReadSeeker) *syncedReader {
	return &syncedReader{
		w:    w,
		r:    r,
		news: make(chan bool),
	}
}

func (s *syncedReader) Write(p []byte) (n int, err error) {
	defer func() {
		written := atomic.AddUint64(&s.written, uint64(n))
		read := atomic.LoadUint64(&s.read)
		if written > read {
			s.wake()
		}
	}()

	n, err = s.w.Write(p)
	return
}

func (s *syncedReader) Read(p []byte) (n int, err error) {
	defer func() { atomic.AddUint64(&s.read, uint64(n)) }()

	for {
		s.sleep()
		n, err = s.r.Read(p)
		if err == io.EOF && !s.isDone() && n == 0 {
			continue
		}

		break
	}

	return
}

func (s *syncedReader) isDone() bool {
	return atomic.LoadUint32(&s.done) == 1
}

func (s *syncedReader) isBlocked() bool {
	return atomic.LoadUint32(&s.blocked) == 1
}

func (s *syncedReader) wake() {
	if s.isBlocked() {
		atomic.StoreUint32(&s.blocked, 0)
		s.news <- true
	}
}

func (s *syncedReader) sleep() {
	read := atomic.LoadUint64(&s.read)
	written := atomic.LoadUint64(&s.written)
	if read >= written {
		atomic.StoreUint32(&s.blocked, 1)
		<-s.news
	}

}

func (s *syncedReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekCurrent {
		return s.r.Seek(offset, whence)
	}

	p, err := s.r.Seek(offset, whence)
	atomic.StoreUint64(&s.read, uint64(p))

	return p, err
}

func (s *syncedReader) Close() error {
	atomic.StoreUint32(&s.done, 1)
	close(s.news)
	return nil
}
