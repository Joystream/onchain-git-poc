package cli

import (
	"bytes"
	"fmt"
	"os"
	"context"

	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
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

type session struct {
	authMethod transport.AuthMethod
	endpoint *transport.Endpoint
	advRefs  *packp.AdvRefs
}

func (*joystreamTransport) NewReceivePackSession(endpoint *transport.Endpoint,
		authMethod transport.AuthMethod) (transport.ReceivePackSession, error) {
	fmt.Fprintf(os.Stderr, "Joystream transport creating ReceivePackSession\n")
	sess := &session{
		authMethod: authMethod,
		endpoint: endpoint,
	}
	return sess, nil
}

func advertisedReferences(s *session, serviceName string) (ref *packp.AdvRefs, err error) {
	advRefs := packp.NewAdvRefs()
	return advRefs, nil
}

func (s *session) AdvertisedReferences() (*packp.AdvRefs, error) {
	fmt.Fprintf(os.Stderr, "Joystream transport getting advertised references\n")
	advRefs, error := advertisedReferences(s, transport.ReceivePackServiceName)
	fmt.Fprintf(os.Stderr, "Joystream transport got advertised references: %v\n", advRefs)
	return advRefs, error
}

func (s *session) ReceivePack(ctx context.Context, req *packp.ReferenceUpdateRequest) (
	*packp.ReportStatus, error) {
	fmt.Fprintf(os.Stderr, "Joystream transport sending reference update request to endpoint\n")

	buf := bytes.NewBuffer(nil)
	if err := req.Encode(buf); err != nil {
		return nil, err
	}

	reportStatus := packp.NewReportStatus()
	reportStatus.CommandStatuses = []*packp.CommandStatus{}
	reportStatus.UnpackStatus = "ok"
	error := reportStatus.Error()
	if error != nil {
		fmt.Fprintf(os.Stderr, "Error making report status: %s\n", error)
		return nil, error
	}

	fmt.Fprintf(os.Stderr, "Returning report status: %v\n", reportStatus)
	return reportStatus, nil
}

func (*session) Close() error {
	return nil
}
