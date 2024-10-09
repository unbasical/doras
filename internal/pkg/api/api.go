package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func BuildApp() *gin.Engine {
	r := gin.Default()
	authV1 := r.Group("/api/v1")
	authV1.POST("edge/artifacts/delta/create", CreateDelta)
	authV1.GET("edge/artifacts/delta", ReadDelta)
	authV1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	return r
}

type CreateDeltaRequestBody struct {
	Identifier string `json:"identifier"`
	Hash       string `json:"hash"`
}

type CreateDeltaResponseBody struct {
	Url string `json:"url"`
}

func CreateDelta(c *gin.Context) {
	// rough idea: look for file at identifier and check against hash. create delta if hash is valid
	var requestBody CreateDeltaRequestBody
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Errorf("failed to parse request body: %s", err)
		c.JSON(http.StatusBadRequest, "missing request body")
		return
	}
	log.Debugf("request body: %+v", requestBody)
	c.JSON(http.StatusOK, CreateDeltaResponseBody{
		Url: "example.org",
	})
	return
}

type ReadDeltaRequestBody struct {
	Identifier string `json:"identifier"`
}

type ReadDeltaResponseBody struct {
}

func ReadDelta(c *gin.Context) {
	var requestBody ReadDeltaRequestBody
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Errorf("failed to parse request body: %s", err)
		c.JSON(http.StatusBadRequest, "missing request body")
		return
	}
	log.Debugf("request body: %+v", requestBody)
	//TODO: return a file or serve as static file?
	c.JSON(http.StatusInternalServerError, "not implemented yet")
	//c.File(path)
}
