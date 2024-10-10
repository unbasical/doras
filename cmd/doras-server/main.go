package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/core"
	"os"
)

func main() {
	log.SetLevel(log.DebugLevel)
	doras := core.Doras{}
	err := os.MkdirAll("./doras-working-dir", 0777)
	if err != nil {
		panic(err)
	}
	doras.Init("./doras-working-dir").Start()
}
