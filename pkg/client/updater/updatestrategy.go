package updater

import (
	"errors"
	"github.com/samber/lo"
	"github.com/unbasical/doras/pkg/client/updater/backupmanager"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/healthchecker"
	"github.com/unbasical/doras/pkg/client/updater/patchapplier"
	"github.com/unbasical/doras/pkg/client/updater/storage"
	"github.com/unbasical/doras/pkg/client/updater/updatefinder"
	"github.com/unbasical/doras/pkg/client/updater/verifier"
)

func update(
	image string,
	finder updatefinder.UpdateFinder,
	s storage.Storage,
	f ociutils.ArtifactFetcher,
	verifiers []verifier.ArtifactVerifier,
	b backupmanager.BackupManager,
	u storage.Updater,
	h healthchecker.HealthChecker,
	forceUpdate bool,
) error {
	currentVersion, isInitialized := s.CurrentVersion()
	isDelta, image, err := finder.LoadUpdate(image, isInitialized)
	if err != nil {
		return NewUpdaterError(ErrTargetImageNotFound, err)
	}
	if u.HasFailed(image) && !forceUpdate {
		return NewUpdaterError(ErrUpdateHasFailedBefore, nil)
	}
	var nextArtifactPath string
	if isDelta {
		deltaDir := s.DeltaDirectory()
		algo, deltaPath, err := f.FetchDelta(currentVersion, image, deltaDir)
		if err != nil {
			return NewUpdaterError(ErrFetchFailed, err)
		}
		applier, err := patchapplier.NewPatchApplier(algo)
		if err != nil {
			return NewUpdaterError(ErrUnknownAlgorithm, err)
		}
		nextArtifactPath, err = applier.Patch(image, deltaPath, s.IntermediateDirectory())
		if err != nil {
			return NewUpdaterError(ErrPatchingFailed, err)
		}
	} else {
		nextArtifactPath, err = f.FetchArtifact(image, s.IntermediateDirectory())
	}

	err = errors.Join(lo.Map(verifiers, func(v verifier.ArtifactVerifier, _ int) error {
		return v.VerifyArtifact(image, nextArtifactPath)
	})...)
	if err != nil {
		u.MarkFailed(image)
		return NewUpdaterError(ErrFailedChecks, err)
	}

	if isInitialized {
		artifactDir := s.ArtifactDirectory()
		err := b.CreateBackup(currentVersion, artifactDir, nextArtifactPath)
		if err != nil {
			return NewUpdaterError(ErrFailedToCreateBackup, err)
		}
	}
	err = u.ApplyUpdate(s.ArtifactDirectory(), nextArtifactPath)
	if err != nil {
		u.MarkFailed(image)
		return NewUpdaterError(ErrFailedToApplyUpdate, errors.Join(err, u.Rollback(b)))
	}
	if err = h.HealthCheck(); err != nil {
		u.MarkFailed(image)
		return NewUpdaterError(ErrFailedHealthChecks, err)
	}
	return nil
}
