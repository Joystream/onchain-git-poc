package gitService

import (
	"bytes"
	"fmt"
	"io"
	stdIOUtil "io/ioutil"
	"os"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/idxfile"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// requireIndex loads packfile indexes
func (pw *PackWriter) requireIndex() error {
	if pw.index != nil {
		return nil
	}

	pw.index = make(map[plumbing.Hash]idxfile.Index)
	if pw.packMap == nil {
		pw.packMap = make(map[plumbing.Hash]struct{})
		pw.packList = nil

		packfileHashes, err := pw.objectPacks()
		if err != nil {
			return err
		}

		for _, h := range packfileHashes {
			pw.packList = append(pw.packList, h)
			pw.packMap[h] = struct{}{}
		}
	}

	for _, h := range pw.packList {
		if err := pw.loadIdx(h); err != nil {
			return err
		}
	}

	return nil
}

// loadIdx loads an index corresponding to a packfile
func (pw *PackWriter) loadIdx(h plumbing.Hash) (err error) {
	path := fmt.Sprintf("%s/objects/packs/pack-%s.idx", pw.repoURI, h)
	b := pw.store.Get([]byte(path))
	if b == nil {
		return fmt.Errorf("Couldn't get index %s", path)
	}

	r := bytes.NewBuffer(b)

	idx := idxfile.NewMemoryIndex()
	d := idxfile.NewDecoder(r)
	if err = d.Decode(idx); err != nil {
		return err
	}

	pw.index[h] = idx
	return err
}

// objectPacks gets hashes of packfiles stored for the repository
func (pw *PackWriter) objectPacks() ([]plumbing.Hash, error) {
	iter := pw.store.Iterator(nil, nil)
	defer iter.Close()
	var packs []plumbing.Hash
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		if strings.HasPrefix(key, fmt.Sprintf("%s/objects/pack/", pw.repoURI)) &&
			strings.HasSuffix(key, ".pack") {
			components := strings.Split(key, "/")
			n := components[len(components)-1]
			// pack-(hash).pack
			h := plumbing.NewHash(n[5 : len(n)-5])
			if h.IsZero() {
				// Ignore files with badly-formatted names.
				continue
			}

			packs = append(packs, h)
		}
	}

	return packs, nil
}

func getPackfileWriter(store sdk.KVStore, repoURI string) (io.WriteCloser, error) {
	fw, err := stdIOUtil.TempFile("", "packfile")
	if err != nil {
		return nil, err
	}
	fr, err := os.Open(fw.Name())
	if err != nil {
		fw.Close()
		return nil, err
	}

	pw := &PackWriter{
		fw:      fw,
		fr:      fr,
		synced:  newSyncedReader(fw, fr),
		result:  make(chan error),
		store:   store,
		repoURI: repoURI,
	}

	if err := pw.requireIndex(); err != nil {
		fmt.Fprintf(os.Stderr, "PackFileWriter - requireIndex failed: %s\n", err)
		fw.Close()
		fr.Close()
		return nil, err
	}

	go pw.buildIndex()

	pw.Notify = func(h plumbing.Hash, w *idxfile.Writer) {
		index, err := w.Index()
		if err == nil {
			pw.index[h] = index
		}
	}

	return pw, nil
}

// PackWriter is an io.Writer that generates an index simultaneously while decoding a packfile.
type PackWriter struct {
	Notify func(plumbing.Hash, *idxfile.Writer)

	fw        *os.File
	fr        *os.File
	synced    *syncedReader
	checksum  plumbing.Hash
	parser    *packfile.Parser
	idxWriter *idxfile.Writer
	result    chan error
	index     map[plumbing.Hash]idxfile.Index
	packList  []plumbing.Hash
	packMap   map[plumbing.Hash]struct{}
	store     sdk.KVStore
	repoURI   string
}

// buildIndex parses the packfile as it gets written and builds an index continuously
func (pw *PackWriter) buildIndex() {
	fmt.Fprintf(os.Stderr, "Building packfile index\n")
	s := packfile.NewScanner(pw.synced)
	pw.idxWriter = new(idxfile.Writer)
	var err error
	pw.parser, err = packfile.NewParser(s, pw.idxWriter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Creating parser failed: %s\n", err)
		pw.result <- err
		return
	}

	checksum, err := pw.parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing packfile failed: %s\n", err)
		pw.result <- err
		return
	}

	pw.checksum = checksum
	fmt.Fprintf(os.Stderr, "Finished parsing packfile\n")
	pw.result <- nil
}

// waitBuildIndex waits until buildIndex function finishes, this can terminate
// with a packfile.ErrEmptyPackfile, this means that nothing was written so we
// ignore the error
func (pw *PackWriter) waitBuildIndex() error {
	fmt.Fprintf(os.Stderr, "Waiting for index to finish building\n")
	err := <-pw.result
	if err == packfile.ErrEmptyPackfile {
		fmt.Fprintf(os.Stderr, "Index finished building due to empty packfile\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Index finished building\n")
	return err
}

func (pw *PackWriter) Write(p []byte) (int, error) {
	fmt.Fprintf(os.Stderr, "Writing %d bytes to packfile\n", len(p))
	return pw.synced.Write(p)
}

// Close closes all the file descriptors and save the final packfile, if nothing
// was written, the tempfiles are deleted without writing a packfile.
func (pw *PackWriter) Close() error {
	fmt.Fprintf(os.Stderr, "Packwriter closing\n")

	defer func() {
		if pw.Notify != nil && pw.idxWriter != nil && pw.idxWriter.Finished() {
			fmt.Fprintf(os.Stderr, "Calling Notify hook\n")
			pw.Notify(pw.checksum, pw.idxWriter)
		}

		close(pw.result)
	}()

	if err := pw.synced.Close(); err != nil {
		return err
	}

	if err := pw.waitBuildIndex(); err != nil {
		return err
	}

	if err := pw.fr.Close(); err != nil {
		return err
	}

	if err := pw.fw.Close(); err != nil {
		return err
	}

	return pw.save()
}

func (pw *PackWriter) save() error {
	fmt.Fprintf(os.Stderr, "Packwriter saving packfile and index\n")
	idxBuf := &bytes.Buffer{}

	idx, err := pw.idxWriter.Index()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Packwriter - getting index failed: %s\n", err)
		return err
	}

	e := idxfile.NewEncoder(idxBuf)
	if _, err := e.Encode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "Packwriter - encoding index failed: %s\n", err)
		return err
	}

	packfilePath := fmt.Sprintf("%s/objects/pack/pack-%s.pack", pw.repoURI, pw.checksum)
	fmt.Fprintf(os.Stderr, "Saving packfile to '%s'\n", packfilePath)
	packfileBytes, err := stdIOUtil.ReadFile(pw.fw.Name())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Reading temporary packfile failed: %s\n", err)
		return err
	}
	os.Remove(pw.fw.Name())

	pw.store.Set([]byte(packfilePath), packfileBytes)

	idxPath := fmt.Sprintf("%s/objects/pack/pack-%s.idx", pw.repoURI, pw.checksum)
	fmt.Fprintf(os.Stderr, "Saving packfile index to '%s'\n", idxPath)
	pw.store.Set([]byte(idxPath), idxBuf.Bytes())

	return nil
}

// writePackfile writes a packfile and its index to a repository
func writePackfile(store sdk.KVStore, msg MsgUpdateReferences) (err error) {
	fmt.Fprintf(os.Stderr, "Keeper - writing packfile and index to %s/objects/pack/\n", msg.URI)
	pw, err := getPackfileWriter(store, msg.URI)
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(pw, &err)
	fmt.Fprintf(os.Stderr, "Copying packfile to packfile writer, %d bytes\n",
		len(msg.Packfile))
	buf := bytes.NewBuffer(msg.Packfile)
	fmt.Fprintf(os.Stderr, "Length of packfile buffer: %d\n", buf.Len())
	_, err = io.Copy(pw, buf)
	fmt.Fprintf(os.Stderr, "Finished copying to packfile writer\n")

	return nil
}
