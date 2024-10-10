package core

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api"
	"github.com/unbasical/doras-server/internal/pkg/storage"
)

type Doras struct {
	storage storage.ArtifactStorage
	engine  *gin.Engine
}

func (d *Doras) Init(storagePath string) *Doras {
	d.storage = &storage.FilesystemStorage{BasePath: storagePath}
	config := api.Config{
		ArtifactStorage: d.storage,
	}
	d.engine = api.BuildApp(&config)
	return d
}

func (d *Doras) Start() *Doras {
	log.Info("Starting doras")

	err := d.engine.Run()
	if err != nil {
		log.Fatal(err)
	}
	return d
}

func (d *Doras) Stop() {
	log.Info("Stopping doras")
	log.Warn("Stop() is not implemented yet")
}
