package cli

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

type joystreamTransport struct {
}

func newJoystreamTransport() joystreamTransport {
	return joystreamTransport{}
}

func (*joystreamTransport) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (
	transport.UploadPackSession, error) {
	fmt.Fprintf(os.Stderr, "Joystream transport creating UploadPackSession\n")
	return nil, nil
}

type rpSession struct {
	storer     storer.Storer
	authMethod transport.AuthMethod
	endpoint   *transport.Endpoint
	advRefs    *packp.AdvRefs
	cmdStatus  map[plumbing.ReferenceName]error
	firstErr   error
	unpackErr  error
}

func (*joystreamTransport) NewReceivePackSession(ep *transport.Endpoint,
	authMethod transport.AuthMethod) (transport.ReceivePackSession, error) {
	repoPath := path.Join("/tmp/joystream/", ep.Path, ".git")
	fmt.Fprintf(os.Stderr,
		"Joystream transport creating ReceivePackSession, storing at '%s'\n", repoPath)
	fs := osfs.New(repoPath)

	if _, err := fs.Stat("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Can't find Git config at '%s'\n", repoPath)
		return nil, transport.ErrRepositoryNotFound
	}

	sto := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())

	sess := &rpSession{
		authMethod: authMethod,
		endpoint:   ep,
		storer:     sto,
		cmdStatus:  map[plumbing.ReferenceName]error{},
	}
	return sess, nil
}

func (s *rpSession) advertisedReferences(serviceName string) (ref *packp.AdvRefs, err error) {
	advRefs := packp.NewAdvRefs()
	return advRefs, nil
}

func (s *rpSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	fmt.Fprintf(os.Stderr, "Joystream transport getting advertised references\n")
	advRefs, error := s.advertisedReferences(transport.ReceivePackServiceName)
	fmt.Fprintf(os.Stderr, "Joystream transport got advertised references: %v\n", advRefs)
	return advRefs, error
}

// ReceivePack receives a ReferenceUpdateRequest, with a packfile stream as its Packfile
// property. The request in turn gets encoded to a binary blob that gets sent to a Joystream
// server, to store on the blockchain.
func (s *rpSession) ReceivePack(ctx context.Context, req *packp.ReferenceUpdateRequest) (
	*packp.ReportStatus, error) {

	fmt.Fprintf(os.Stderr, "Joystream transport sending reference update request to endpoint\n")

	// TODO: Make references update atomic

	r := ioutil.NewContextReadCloser(ctx, req.Packfile)
	fmt.Fprintf(os.Stderr, "Updating object storage\n")
	if err := packfile.UpdateObjectStorage(s.storer, r); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update object storage: %s\n", err)
		r.Close()
		return s.reportStatus(), err
	}
	if err := r.Close(); err != nil {
		return s.reportStatus(), err
	}

	s.updateReferences(req)

	return s.reportStatus(), nil

	// Encode as blob to send to server
	// buf := bytes.NewBuffer(nil)
	// if err := req.Encode(buf); err != nil {
	// 	return nil, err
	// }

	// req := packp.NewReferenceUpdateRequest()
	// if err := req.Decode(buf); err != nil {
	// 	return fmt.Errorf("error decoding: %s", err)
	// }

	// reportStatus := packp.NewReportStatus()
	// reportStatus.CommandStatuses = []*packp.CommandStatus{}
	// reportStatus.UnpackStatus = "ok"
	// error := reportStatus.Error()
	// if error != nil {
	// 	fmt.Fprintf(os.Stderr, "Error making report status: %s\n", error)
	// 	return nil, error
	// }

	// fmt.Fprintf(os.Stderr, "Returning report status: %v\n", reportStatus)
	// return reportStatus, nil
}

func (s *rpSession) reportStatus() *packp.ReportStatus {
	rs := packp.NewReportStatus()
	rs.UnpackStatus = "ok"

	if s.unpackErr != nil {
		rs.UnpackStatus = s.unpackErr.Error()
	}

	if s.cmdStatus == nil {
		return rs
	}

	for ref, err := range s.cmdStatus {
		msg := "ok"
		if err != nil {
			msg = err.Error()
		}
		status := &packp.CommandStatus{
			ReferenceName: ref,
			Status:        msg,
		}
		rs.CommandStatuses = append(rs.CommandStatuses, status)
	}

	return rs
}

func (s *rpSession) setStatus(ref plumbing.ReferenceName, err error) {
	s.cmdStatus[ref] = err
	if s.firstErr == nil && err != nil {
		s.firstErr = err
	}
}

func (s *rpSession) updateReferences(req *packp.ReferenceUpdateRequest) {
	errUpdateReference := errors.New("failed to update ref")

	fmt.Fprintf(os.Stderr, "Updating references\n")
	for _, cmd := range req.Commands {
		exists, err := referenceExists(s.storer, cmd.Name)
		if err != nil {
			s.setStatus(cmd.Name, err)
			continue
		}

		switch cmd.Action() {
		case packp.Create:
			if exists {
				s.setStatus(cmd.Name, errUpdateReference)
				continue
			}

			ref := plumbing.NewHashReference(cmd.Name, cmd.New)
			err := s.storer.SetReference(ref)
			s.setStatus(cmd.Name, err)
		case packp.Delete:
			if !exists {
				s.setStatus(cmd.Name, errUpdateReference)
				continue
			}

			err := s.storer.RemoveReference(cmd.Name)
			s.setStatus(cmd.Name, err)
		case packp.Update:
			if !exists {
				s.setStatus(cmd.Name, errUpdateReference)
				continue
			}

			if err != nil {
				s.setStatus(cmd.Name, err)
				continue
			}

			ref := plumbing.NewHashReference(cmd.Name, cmd.New)
			err := s.storer.SetReference(ref)
			s.setStatus(cmd.Name, err)
		}
	}
}

func (*rpSession) Close() error {
	return nil
}

func referenceExists(s storer.ReferenceStorer, n plumbing.ReferenceName) (bool, error) {
	_, err := s.Reference(n)
	if err == plumbing.ErrReferenceNotFound {
		return false, nil
	}

	return err == nil, err
}
