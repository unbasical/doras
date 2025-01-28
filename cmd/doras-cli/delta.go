package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/client"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/client/edgeapi"
	"net/url"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func (args *cliArgs) readDelta(ctx context.Context) error {
	_ = ctx

	// try to load credentials
	creds, err := args.getCredentialFunc()
	if err != nil {
		return err
	}
	var tokenProvider client.AuthProvider
	tokenProvider, err = newCredentialFuncTokenProvider(creds, args.ReadDelta.From)
	if err != nil {
		log.WithError(err).Info("did not load any auth token providers")
	}
	dorasClient, err := edgeapi.NewEdgeClient(
		args.Remote,
		args.InsecureAllowHTTP,
		tokenProvider,
	)
	if err != nil {
		return err
	}
	var res *apicommon.ReadDeltaResponse
	// If the async flag is not set we do not block.
	if args.ReadDelta.Async {
		log.Info("asynchronously requesting delta")
		var exists bool
		res, exists, err = dorasClient.ReadDeltaAsync(args.ReadDelta.From, args.ReadDelta.To, nil)
		if err != nil {
			return err
		}
		if !exists {
			log.Info("delta has not been created yet")
			return nil
		}
	} else {
		res, err = dorasClient.ReadDelta(args.ReadDelta.From, args.ReadDelta.To, nil)
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

type credentialFuncTokenProvider struct {
	auth.CredentialFunc
	registry url.URL
}

// newCredentialFuncTokenProvider creates a token provider that uses the provided auth.CredentialFunc to load access tokens.
// Only works if the credential function is token based.
func newCredentialFuncTokenProvider(creds auth.CredentialFunc, image string) (client.AuthProvider, error) {
	serverUrl, err := ociutils.ParseOciUrl(image)
	if err != nil {
		return nil, err
	}
	return &credentialFuncTokenProvider{
		CredentialFunc: creds,
		registry:       *serverUrl,
	}, nil
}

func (c *credentialFuncTokenProvider) GetAuth() (auth.Credential, error) {
	serverUrl := c.registry.Host
	credential, err := c.CredentialFunc(context.Background(), serverUrl)
	if err != nil {
		return auth.Credential{}, err
	}
	return credential, nil
}
