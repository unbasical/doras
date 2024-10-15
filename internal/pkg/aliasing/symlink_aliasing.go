package aliasing

import (
	"errors"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/internal/pkg/utils"
)

type SymlinkAliasing struct {
	BasePath string
}

func (aliaser *SymlinkAliasing) ResolveAlias(alias string) (string, error) {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	aliasPathClean, err := utils.VerifyPath(aliasPath, aliaser.BasePath)
	if err != nil {
		return "", err
	}
	identifier, err := os.Readlink(aliasPathClean)
	if err != nil {
		return "", errors.New("could not resolve alias: " + alias)
	}
	return identifier, nil
}

func (aliaser *SymlinkAliasing) AddAlias(alias, target string) error {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	// verify the alias path, assume target is trustworthy
	aliasPathClean, err := utils.VerifyPath(aliasPath, aliaser.BasePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(aliasPathClean); err == nil {
		return dorasErrors.ErrAliasExists
	} else if errors.Is(err, os.ErrNotExist) {
		log.Debugf("creating symlink from %s to %s", alias, target)
		// do not use the base path for oldname because the bath is relative to the symlink
		if errInner := os.Symlink(target, aliasPathClean); errInner != nil {
			return errInner
		}
		return nil
	} else {
		log.Fatalf("unexpected error while checking if %s exists", target)
		panic(err)
	}
}
