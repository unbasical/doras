package aliasing

import (
	"errors"
	log "github.com/sirupsen/logrus"
	error2 "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/internal/pkg/utils"
	"os"
	"path/filepath"
	"sync"
)

type SymlinkAliasing struct {
	BasePath string
	lock     sync.RWMutex
}

func (aliaser *SymlinkAliasing) ResolveAlias(alias string) (string, error) {
	aliaser.lock.RLock()
	defer aliaser.lock.RUnlock()
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
	aliaser.lock.Lock()
	defer aliaser.lock.Unlock()
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	// verify the alias path, assume target is trustworthy
	aliasPathClean, err := utils.VerifyPath(aliasPath, aliaser.BasePath, false)
	if err != nil {
		return err
	}
	if _, err := os.Stat(aliasPathClean); err == nil {
		return error2.DorasAliasExistsError
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
