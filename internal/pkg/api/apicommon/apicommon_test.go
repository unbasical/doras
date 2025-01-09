package apicommon

import (
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"testing"
)

func TestParseOciImageString(t *testing.T) {
	tests := []struct {
		name         string
		image        string
		wantRepoName string
		wantTag      string
		wantIsDigest bool
		wantErr      bool
	}{
		{
			name:         "port and tag",
			image:        "localhost:8000/hello:latest",
			wantRepoName: "localhost:8000/hello",
			wantTag:      "latest",
			wantIsDigest: false,
			wantErr:      false,
		},
		{
			name:         "port and digest",
			image:        "localhost:8000/hello@sha256:49097eb204ddd2189b0b93e087df398bd60ee6edf179ffbe40a2cc54d65e40f8",
			wantRepoName: "localhost:8000/hello",
			wantTag:      "@sha256:49097eb204ddd2189b0b93e087df398bd60ee6edf179ffbe40a2cc54d65e40f8",
			wantIsDigest: true,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepoName, gotTag, gotIsDigest, err := ociutils.ParseOciImageString(tt.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOciImageString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRepoName != tt.wantRepoName {
				t.Errorf("ParseOciImageString() gotRepoName = %v, want %v", gotRepoName, tt.wantRepoName)
			}
			if gotTag != tt.wantTag {
				t.Errorf("ParseOciImageString() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
			if gotIsDigest != tt.wantIsDigest {
				t.Errorf("ParseOciImageString() gotIsDigest = %v, want %v", gotIsDigest, tt.wantIsDigest)
			}
		})
	}
}
