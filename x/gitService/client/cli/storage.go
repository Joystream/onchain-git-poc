package cli

import (
	// "encoding/json"
	// "fmt"
	"io"
	// "io/ioutil"

	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/index"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

const (
	urlField      = "url"
	referencesSet = "reference"
	configSet     = "config"
	typesSet      = "types"
)

type gitStorage struct {
	ns     string
	url    string
}

func newGitStorage(ns, url string) (*gitStorage, error) {
	return &gitStorage{ns, url,}, nil
}

func (s *gitStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

func (s *gitStorage) SetEncodedObject(obj plumbing.EncodedObject) (plumbing.Hash, error) {
	// key, err := s.buildObjectKey(obj.Hash(), obj.Type())
	// if err != nil {
	// 	return obj.Hash(), err
	// }

	// r, err := obj.Reader()
	// if err != nil {
	// 	return obj.Hash(), err
	// }

	// c, err := ioutil.ReadAll(r)
	// if err != nil {
	// 	return obj.Hash(), err
	// }
	//
	// bins := driver.BinMap{
	// 	urlField: s.url,
	// 	"hash":   obj.Hash().String(),
	// 	"type":   obj.Type().String(),
	// 	"blob":   c,
	// }

	// if err := s.setEncodedObjectType(obj); err != nil {
	// 	return obj.Hash(), err
	// }

	// err = s.client.Put(nil, key, bins)
	return obj.Hash(), nil
}

func (s *gitStorage) setEncodedObjectType(obj plumbing.EncodedObject) error {
	// key, err := s.buildTypeKey(obj.Hash())
	// if err != nil {
	// 	return err
	// }
	//
	// bins := driver.BinMap{
	// 	"type": obj.Type().String(),
	// }
	//
	// return s.client.Put(nil, key, bins)
	return nil
}

func (s *gitStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	var err error
	if t == plumbing.AnyObject {
		t, err = s.encodedObjectType(h)
		if err != nil {
			return nil, err
		}
	}

	// key, err := s.buildObjectKey(h, t)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// if rec == nil {
	// 	return nil, plumbing.ErrObjectNotFound
	// }
	//
	// return objectFromRecord(rec, t)

	return nil, plumbing.ErrObjectNotFound
}

func (s *gitStorage) encodedObjectType(h plumbing.Hash) (plumbing.ObjectType, error) {
	return plumbing.AnyObject, nil
	// key, err := s.buildTypeKey(h)
	// if err != nil {
	// 	return plumbing.AnyObject, err
	// }
	//
	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return plumbing.AnyObject, err
	// }
	//
	// if rec == nil {
	// 	return plumbing.AnyObject, plumbing.ErrObjectNotFound
	// }
	//
	// return plumbing.ParseObjectType(rec.Bins["type"].(string))
}

func (s *gitStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	// stmnt := driver.NewStatement(s.ns, t.String())
	// if err := stmnt.Addfilter(driver.NewEqualFilter(urlField, s.url)); err != nil {
	// 	return err
	// }
	//
	// rs, err := s.client.Query(nil, stmnt)
	// if err != nil {
	// 	return nil, err
	// }

	// return &EncodedObjectIter{t, rs.Records}, nil
	return &EncodedObjectIter{t}, nil
}

// EncodedObjectIter ...
type EncodedObjectIter struct {
	t  plumbing.ObjectType
}

// Next implements storer.EncodedObjectIter
func (i *EncodedObjectIter) Next() (plumbing.EncodedObject, error) {
	// r := <-i.ch
	// if r == nil {
	// 	return nil, io.EOF
	// }

	return nil, io.EOF
	// return objectFromRecord(r, i.t)
}

// ForEach implements storer.EncodedObjectIter
func (i *EncodedObjectIter) ForEach(cb func(obj plumbing.EncodedObject) error) error {
	for {
		obj, err := i.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if err := cb(obj); err != nil {
			if err == storer.ErrStop {
				return nil
			}

			return err
		}
	}
}

// Close implements storer.EncodedObjectIter
func (i *EncodedObjectIter) Close() {}

// func objectFromRecord(r *driver.Record, t plumbing.ObjectType) (plumbing.EncodedObject, error) {
// 	content := r.Bins["blob"].([]byte)
//
// 	o := &plumbing.MemoryObject{}
// 	o.SetType(t)
// 	o.SetSize(int64(len(content)))
//
// 	_, err := o.Write(content)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return o, nil
// }

func (s *gitStorage) SetReference(ref *plumbing.Reference) error {
	// key, err := s.buildReferenceKey(ref.Name())
	// if err != nil {
	// 	return err
	// }

	// raw := ref.Strings()
	// bins := driver.BinMap{
	// 	urlField: s.url,
	// 	"name":   raw[0],
	// 	"target": raw[1],
	// }

	// return s.client.Put(nil, key, bins)
	return nil
}

func (s *gitStorage) Reference(n plumbing.ReferenceName) (*plumbing.Reference, error) {
	// key, err := s.buildReferenceKey(n)
	// if err != nil {
	// 	return nil, err
	// }

	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// if rec == nil {
	// 	return nil, plumbing.ErrReferenceNotFound
	// }
	//
	// return plumbing.NewReferenceFromStrings(
	// 	rec.Bins["name"].(string),
	// 	rec.Bins["target"].(string),
	// ), nil

	return nil, plumbing.ErrReferenceNotFound
}

// func (s *gitStorage) buildReferenceKey(n plumbing.ReferenceName) (*driver.Key, error) {
// 	return driver.NewKey(s.ns, referencesSet, fmt.Sprintf("%s|%s", s.url, n))
// }

func (s *gitStorage) IterReferences() (storer.ReferenceIter, error) {
	// stmnt := driver.NewStatement(s.ns, referencesSet)
	// err := stmnt.Addfilter(driver.NewEqualFilter(urlField, s.url))
	// if err != nil {
	// 	return nil, err
	// }
	//
	// rs, err := s.client.Query(nil, stmnt)
	// if err != nil {
	// 	return nil, err
	// }

	var refs []*plumbing.Reference
	// for r := range rs.Records {
	// 	refs = append(refs, plumbing.NewReferenceFromStrings(
	// 		r.Bins["name"].(string),
	// 		r.Bins["target"].(string),
	// 	))
	// }

	return storer.NewReferenceSliceIter(refs), nil
}

func (s *gitStorage) Config() (*config.Config, error) {
	// key, err := s.buildConfigKey()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// if rec == nil {
	// 	return config.NewConfig(), nil
	// }
	//
	// c := &config.Config{}
	// return c, json.Unmarshal(rec.Bins["blob"].([]byte), c)

	c := &config.Config{}
	return c, nil
}

func (s *gitStorage) SetConfig(r *config.Config) error {
	// key, err := s.buildConfigKey()
	// if err != nil {
	// 	return err
	// }
	//
	// json, err := json.Marshal(r)
	// if err != nil {
	// 	return err
	// }
	//
	// bins := driver.BinMap{
	// 	urlField: s.url,
	// 	"blob":   json,
	// }
	//
	// return s.client.Put(nil, key, bins)
	return nil
}

// func (s *gitStorage) buildConfigKey() (*driver.Key, error) {
// 	return driver.NewKey(s.ns, configSet, fmt.Sprintf("%s|config", s.url))
// }

func (s *gitStorage) Index() (*index.Index, error) {
	// key, err := s.buildIndexKey()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return nil, err
	// }

	idx := &index.Index{}
	// return idx, json.Unmarshal(rec.Bins["blob"].([]byte), idx)
	return idx, nil
}

func (s *gitStorage) SetIndex(idx *index.Index) error {
	// key, err := s.buildIndexKey()
	// if err != nil {
	// 	return err
	// }
	//
	// json, err := json.Marshal(idx)
	// if err != nil {
	// 	return err
	// }
	//
	// bins := driver.BinMap{
	// 	urlField: s.url,
	// 	"blob":   json,
	// }
	//
	// return s.client.Put(nil, key, bins)
	return nil
}

// func (s *gitStorage) buildIndexKey() (*driver.Key, error) {
// 	return driver.NewKey(s.ns, configSet, fmt.Sprintf("%s|index", s.url))
// }

func (s *gitStorage) Shallow() ([]plumbing.Hash, error) {
	// key, err := s.buildShallowKey()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// rec, err := s.client.Get(nil, key)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// var h []plumbing.Hash
	// return h, json.Unmarshal(rec.Bins["blob"].([]byte), h)
	return []plumbing.Hash{}, nil
}

func (s *gitStorage) SetShallow(hash []plumbing.Hash) error {
	// key, err := s.buildShallowKey()
	// if err != nil {
	// 	return err
	// }
	//
	// json, err := json.Marshal(hash)
	// if err != nil {
	// 	return err
	// }
	//
	// bins := driver.BinMap{
	// 	urlField: s.url,
	// 	"blob":   json,
	// }
	//
	// return s.client.Put(nil, key, bins)
	return nil
}

// func (s *gitStorage) buildShallowKey() (*driver.Key, error) {
// 	return driver.NewKey(s.ns, configSet, fmt.Sprintf("%s|shallow", s.url))
// }
