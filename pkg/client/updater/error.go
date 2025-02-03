package updater

import "fmt"

type UpdaterError struct {
	cause error
	kind  error
}

func (u UpdaterError) Error() string {
	return fmt.Sprintf("kind: %q, cause: %q", u.kind.Error(), u.cause.Error())
}

func NewUpdaterError(kind error, cause error) error {
	return UpdaterError{
		cause: cause,
		kind:  kind,
	}
}

var (
	ErrTargetImageNotFound   = fmt.Errorf("target image not found")
	ErrFetchFailed           = fmt.Errorf("failed to fetch image")
	ErrFailedChecks          = fmt.Errorf("failed checks")
	ErrFailedToApplyUpdate   = fmt.Errorf("failed to apply update")
	ErrFailedToCreateBackup  = fmt.Errorf("failed to create backup")
	ErrPatchingFailed        = fmt.Errorf("failed to patch")
	ErrUnknownAlgorithm      = fmt.Errorf("unknown algorithm")
	ErrFailedHealthChecks    = fmt.Errorf("failed health checks")
	ErrUpdateHasFailedBefore = fmt.Errorf("failed to update has failed")
)
