package storage

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type Aliasing interface {
	AddAlias(alias string, target string) error
	ResolveAlias(alias string) (string, error)
}

type SymlinkAliasing struct {
	BasePath string
}

func (aliaser *SymlinkAliasing) ResolveAlias(alias string) (string, error) {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	identifier, err := os.Readlink(aliasPath)
	if err != nil {
		return "", errors.New("could not resolve alias: " + alias)
	}
	return identifier, nil
}

func (aliaser *SymlinkAliasing) AddAlias(alias string, target string) error {
	aliasPath := filepath.Join(aliaser.BasePath, alias)
	if _, err := os.Stat(aliasPath); err == nil {
		return errors.New("symlink already exists")
	} else if errors.Is(err, os.ErrNotExist) {
		log.Debugf("creating symlink from %s to %s", alias, target)
		// do not use the base path for oldname because the bath is relative to the symlink
		if err := os.Symlink(target, aliasPath); err != nil {
			return err
		}
		return nil
	} else {
		log.Fatalf("unexpected error while checking if %s exists", target)
		panic(err)
	}
}
