package cli

import (
	"bytes"
	"context"
	encJson "encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"

	cosmosContext "github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/utils"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/joystream/onchain-git-poc/x/gitService"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

type joystreamClient struct {
	ep         *transport.Endpoint
	txBldr     authtxb.TxBuilder
	cliCtx     cosmosContext.CLIContext
	author     sdk.AccAddress
	moduleName string
}

var reRepoURI = regexp.MustCompile("^[^/]+/[^/]+$")

func newJoystreamClient(uri string, cliCtx cosmosContext.CLIContext, txBldr authtxb.TxBuilder,
	author sdk.AccAddress, moduleName string) (*joystreamClient, error) {
	if !reRepoURI.MatchString(uri) {
		return nil, fmt.Errorf("Repo URI on invalid format: '%s'", uri)
	}

	url := fmt.Sprintf("joystream://blockchain/%s", uri)
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create endpoint for URL '%s'\n", url)
		return nil, err
	}
	return &joystreamClient{
		ep:         ep,
		txBldr:     txBldr,
		cliCtx:     cliCtx,
		author:     author,
		moduleName: moduleName,
	}, nil
}

func (*joystreamClient) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (
	transport.UploadPackSession, error) {
	fmt.Fprintf(os.Stderr, "Joystream client creating UploadPackSession\n")
	return nil, nil
}

type rpSession struct {
	authMethod transport.AuthMethod
	endpoint   *transport.Endpoint
	advRefs    *packp.AdvRefs
	cmdStatus  map[plumbing.ReferenceName]error
	firstErr   error
	unpackErr  error
	client     *joystreamClient
}

func (c *joystreamClient) NewReceivePackSession(ep *transport.Endpoint,
	authMethod transport.AuthMethod) (transport.ReceivePackSession, error) {
	fmt.Fprintf(os.Stderr, "Joystream client creating ReceivePackSession\n")

	sess := &rpSession{
		authMethod: authMethod,
		endpoint:   ep,
		cmdStatus:  map[plumbing.ReferenceName]error{},
		client:     c,
	}
	return sess, nil
}

func (s *rpSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	fmt.Fprintf(os.Stderr, "Joystream client getting advertised references\n")

	queryPath := fmt.Sprintf("custom/%s/advertisedReferences/%s", s.client.moduleName,
		s.client.ep.Path[1:])
	fmt.Fprintf(os.Stderr, "Joystream client making query, path: '%s'", queryPath)
	res, err := s.client.cliCtx.QueryWithData(queryPath, nil)
	if err != nil {
		return nil, err
	}

	var advRefs *packp.AdvRefs
	if err := encJson.Unmarshal(res, &advRefs); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "Got advertised references from server: %v\n", advRefs.References)

	fmt.Fprintf(os.Stderr, "Joystream client got advertised references: %v\n", advRefs)
	return advRefs, nil
}

// ReceivePack receives a ReferenceUpdateRequest, with a packfile stream as its Packfile
// property. The request in turn gets encoded to a binary blob that gets sent to a Joystream
// server, to store on the blockchain.
func (s *rpSession) ReceivePack(ctx context.Context, req *packp.ReferenceUpdateRequest) (
	*packp.ReportStatus, error) {

	fmt.Fprintf(os.Stderr, "Joystream client sending reference update request to endpoint\n")

	// TODO: Make references update atomic

	fmt.Fprintf(os.Stderr, "Joystream client encoding packfile...\n")
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, req.Packfile); err != nil {
		fmt.Fprintf(os.Stderr, "Joystream client failed to encode packfile: %s\n", err)
		req.Packfile.Close()
		return s.reportStatus(), err
	}
	if err := req.Packfile.Close(); err != nil {
		return s.reportStatus(), err
	}

	repoURI := s.endpoint.Path[1:]
	fmt.Fprintf(os.Stderr, "Creating MsgUpdateReferences, repo URI: '%s'\n", s.endpoint.Path)
	msg, err := gitService.NewMsgUpdateReferences(repoURI, req, buf.Bytes(),
		s.client.author)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Joystream client failed to create MsgUpdateReferences: %s\n", err)
		return s.reportStatus(), err
	}
	fmt.Fprintf(os.Stderr,
		"Joystream client sending MsgUpdateReferences to server for repo '%s' with %d command(s)\n",
		msg.URI, len(msg.Commands))

	if err := utils.CompleteAndBroadcastTxCli(s.client.txBldr, s.client.cliCtx,
		[]sdk.Msg{msg}); err != nil {
		fmt.Fprintf(os.Stderr, "Sending MsgUpdateReferences to node failed: %s\n", err)
		return s.reportStatus(), err
	}

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

func (*rpSession) Close() error {
	return nil
}
