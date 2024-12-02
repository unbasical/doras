package cloudapi

import (
	"bytes"
	"encoding/json"
	"github.com/unbasical/doras-server/internal/pkg/utils/buildurl"

	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/api/cloudapi"
)

func (c *Client) CreateArtifactFromOCIReference(image string) (string, string, error) {
	request := apicommon.CreateOCIArtifactRequest{
		Image: image,
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", "", err
	}

	reqUrl := buildurl.New(
		buildurl.WithBasePath(c.base.DorasURL),
		buildurl.WithPathElement(apicommon.ApiBasePath),
		buildurl.WithPathElement(apicommon.ArtifactsApiPath),
		buildurl.WithQueryParam(apicommon.ArtifactSourceParamKey, apicommon.ArtifactSourceParamValueOci),
	)
	res, err := c.base.Client.Post(reqUrl, "application/json", bytes.NewReader(requestJSON))
	if err != nil {
		return "", "", err
	}
	var resParsed apicommon.SuccessResponse[cloudapi.CreateArtifactResponse]
	err = json.NewDecoder(res.Body).Decode(&resParsed)
	if err != nil {
		return "", "", err
	}
	return resParsed.Success.Path, resParsed.Success.Tag, err
}
