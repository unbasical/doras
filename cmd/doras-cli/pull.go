package main

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/pkg/client/updater"
)

// pull image from the registry and use delta updates if possible.
func (args *cliArgs) pull(ctx context.Context) error {
	client, err := updater.NewClient(
		updater.WithRemoteURL(args.Remote),
		updater.WithInternalDirectory(args.Pull.InternalDir),
		updater.WithOutputDirectory(args.Pull.Output),
		updater.WithDockerConfigPath(args.DockerConfigFilePath),
		updater.WithAcceptedAlgorithms(args.Pull.AcceptedAlgorithm),
		updater.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	if args.Pull.Async {
		// this does not block until the delta has been created
		log.Info("running doras pull asynchronously")
		exists, err := client.PullAsync(args.Pull.Image)
		if err != nil {
			return err
		}
		// print this to allow users to parse the results when there is no error
		if exists {
			log.Info("successfully applied delta")
			return nil
		}
		log.Info("delta request pending, exiting")
		return nil
	}
	// this blocks until the delta has been created
	log.Info("running doras pull synchronously")
	err = client.Pull(args.Pull.Image)
	if err != nil {
		return err
	}
	log.Info("successfully pulled delta")
	return nil
}
