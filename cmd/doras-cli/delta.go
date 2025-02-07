package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/pkg/client/edgeapi"
)

func (args *cliArgs) readDelta(ctx context.Context) error {
	_ = ctx

	// try to load credentials
	creds, err := args.getCredentialFunc()
	if err != nil {
		return err
	}
	dorasClient, err := edgeapi.NewEdgeClient(
		args.Remote,
		args.InsecureAllowHTTP,
		creds,
	)
	if err != nil {
		return err
	}
	var res *apicommon.ReadDeltaResponse
	// If the async flag is not set we do not block.
	if args.ReadDelta.Async {
		log.Info("asynchronously requesting delta")
		var exists bool
		res, exists, err = dorasClient.ReadDeltaAsync(args.ReadDelta.From, args.ReadDelta.To, args.ReadDelta.AcceptedAlgorithm)
		if err != nil {
			return err
		}
		if !exists {
			log.Info("delta has not been created yet")
			return nil
		}
	} else {
		res, err = dorasClient.ReadDelta(args.ReadDelta.From, args.ReadDelta.To, args.ReadDelta.AcceptedAlgorithm)
		if err != nil {
			return err
		}
	}
	// print server response
	resJSON, err := json.Marshal(res)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", resJSON)
	return nil
}
