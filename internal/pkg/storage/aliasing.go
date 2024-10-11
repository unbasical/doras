package storage

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"os"
	"path/filepath"
)

type Aliasing interface {
	// AddAlias
	// The implementation has to handle the sanitization of the identifier.
	AddAlias(alias string, target string) error
	// ResolveAlias
	// The implementation has to handle the sanitization of the identifier.
	ResolveAlias(alias string) (string, error)
}

type SymlinkAliasing struct {
	BasePath string
}

func (aliaser *SymlinkAliasing) ResolveAlias(alias string) (string, error) {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	aliasPathClean, err := utils.VerifyPath(aliasPath, aliaser.BasePath, true)
	if err != nil {
		return "", err
	}
	identifier, err := os.Readlink(aliasPathClean)
	if err != nil {
		return "", errors.New("could not resolve alias: " + alias)
	}
	return identifier, nil
}

func (aliaser *SymlinkAliasing) AddAlias(alias string, target string) error {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	// verify the alias path, assume target is trustworthy
	aliasPathClean, err := utils.VerifyPath(aliasPath, aliaser.BasePath, false)
	if err != nil {
		return err
	}
	if _, err := os.Stat(aliasPathClean); err == nil {
		return errors.New("symlink already exists")
	} else if errors.Is(err, os.ErrNotExist) {
		log.Debugf("creating symlink from %s to %s", alias, target)
		// do not use the base path for oldname because the bath is relative to the symlink
		if err := os.Symlink(target, aliasPathClean); err != nil {
			return err
		}
		return nil
	} else {
		log.Fatalf("unexpected error while checking if %s exists", target)
		panic(err)
	}
}
