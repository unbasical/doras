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

func (d *Doras) Start() {
	log.Info("Starting doras")
	d.engine = api.BuildApp()
	err := d.engine.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func (d *Doras) Stop() {
	log.Info("Stopping doras")
	log.Warn("Stop() is not implemented yet")
}
