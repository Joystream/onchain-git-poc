package gitService

import (
	"bytes"
	"fmt"
	"io"
	stdIOUtil "io/ioutil"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
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
	path := fmt.Sprintf("%s/objects/pack/pack-%s.idx", pw.repoURI, h)
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
		log.Debug().Msgf("PackFileWriter - requireIndex failed: %s", err)
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
	log.Debug().Msgf("Building packfile index")
	s := packfile.NewScanner(pw.synced)
	pw.idxWriter = new(idxfile.Writer)
	var err error
	pw.parser, err = packfile.NewParser(s, pw.idxWriter)
	if err != nil {
		log.Debug().Msgf("Creating parser failed: %s", err)
		pw.result <- err
		return
	}

	checksum, err := pw.parser.Parse()
	if err != nil {
		log.Debug().Msgf("Parsing packfile failed: %s", err)
		pw.result <- err
		return
	}

	pw.checksum = checksum
	log.Debug().Msgf("Finished parsing packfile")
	pw.result <- nil
}

// waitBuildIndex waits until buildIndex function finishes, this can terminate
// with a packfile.ErrEmptyPackfile, this means that nothing was written so we
// ignore the error
func (pw *PackWriter) waitBuildIndex() error {
	log.Debug().Msgf("Waiting for index to finish building")
	err := <-pw.result
	if err == packfile.ErrEmptyPackfile {
		log.Debug().Msgf("Index finished building due to empty packfile")
		return nil
	}

	log.Debug().Msgf("Index finished building")
	return err
}

func (pw *PackWriter) Write(p []byte) (int, error) {
	log.Debug().Msgf("Writing %d bytes to packfile", len(p))
	return pw.synced.Write(p)
}

// Close closes all the file descriptors and save the final packfile, if nothing
// was written, the tempfiles are deleted without writing a packfile.
func (pw *PackWriter) Close() error {
	log.Debug().Msgf("Packwriter closing")

	defer func() {
		if pw.Notify != nil && pw.idxWriter != nil && pw.idxWriter.Finished() {
			log.Debug().Msgf("Calling Notify hook")
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
	log.Debug().Msgf("Packwriter saving packfile and index")
	idxBuf := &bytes.Buffer{}

	idx, err := pw.idxWriter.Index()
	if err != nil {
		log.Debug().Msgf("Packwriter - getting index failed: %s", err)
		return err
	}

	e := idxfile.NewEncoder(idxBuf)
	if _, err := e.Encode(idx); err != nil {
		log.Debug().Msgf("Packwriter - encoding index failed: %s", err)
		return err
	}

	packfilePath := fmt.Sprintf("%s/objects/pack/pack-%s.pack", pw.repoURI, pw.checksum)
	log.Debug().Msgf("Saving packfile to '%s'", packfilePath)
	packfileBytes, err := stdIOUtil.ReadFile(pw.fw.Name())
	if err != nil {
		log.Debug().Msgf("Reading temporary packfile failed: %s", err)
		return err
	}
	os.Remove(pw.fw.Name())

	pw.store.Set([]byte(packfilePath), packfileBytes)

	idxPath := fmt.Sprintf("%s/objects/pack/pack-%s.idx", pw.repoURI, pw.checksum)
	log.Debug().Msgf("Saving packfile index to '%s'", idxPath)
	pw.store.Set([]byte(idxPath), idxBuf.Bytes())

	return nil
}

// writePackfile writes a packfile and its index to a repository
func writePackfile(store sdk.KVStore, msg MsgUpdateReferences) (err error) {
	log.Debug().Msgf("Keeper - writing packfile and index to %s/objects/pack/", msg.URI)
	pw, err := getPackfileWriter(store, msg.URI)
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(pw, &err)
	log.Debug().Msgf("Copying packfile to packfile writer, %d bytes\n",
		len(msg.Packfile))
	buf := bytes.NewBuffer(msg.Packfile)
	log.Debug().Msgf("Length of packfile buffer: %d", buf.Len())
	_, err = io.Copy(pw, buf)
	log.Debug().Msgf("Finished copying to packfile writer")

	return nil
}
