package client

import (
	"bytes"
	"encoding/json"

	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/api/cloudapi"
)

type CloudAPIClient struct {
	base *dorasBaseClient
}

func NewCloudClient(serverURL string) *CloudAPIClient {
	return &CloudAPIClient{base: newBaseClient(serverURL)}
}

func (c *CloudAPIClient) CreateArtifactFromOCIReference(image string) (string, string, error) {
	request := cloudapi.CreateOCIArtifactRequest{
		Image: image,
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", "", err
	}
	res, err := c.base.client.Post(c.base.serverURL+"/api/artifacts?from=oci", "application/json", bytes.NewReader(requestJSON))
	if err != nil {
		panic(err)
	}
	var resParsed apicommon.SuccessResponse[cloudapi.CreateArtifactResponse]
	err = json.NewDecoder(res.Body).Decode(&resParsed)
	if err != nil {
		return "", "", err
	}
	return resParsed.Success.Path, resParsed.Success.Tag, err
}
