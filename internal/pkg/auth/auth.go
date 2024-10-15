package auth

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func AlwaysAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Warn("AlwaysAuth: accepted")
	}
}
