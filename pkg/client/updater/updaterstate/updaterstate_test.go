package updaterstate

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	"testing"
)

// TestSetArtifactState_Success tests if SetArtifactState correctly stores a digest.
func TestSetArtifactState_Success(t *testing.T) {

	state := State{
		Version:        "1.0",
		ArtifactStates: make(map[string]string),
	}
	artifactDir := "/artifacts/path"
	image := "valid.repo/image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err := state.SetArtifactState(artifactDir, image)
	if err != nil {
		t.Fatalf("SetArtifactState failed: %v", err)
	}

	expectedKey := "(%s,%s)"
	expectedKey = fmt.Sprintf(expectedKey, artifactDir, "valid.repo/image")
	expectedDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	if storedDigest, exists := state.ArtifactStates[expectedKey]; !exists || storedDigest != expectedDigest {
		t.Errorf("Expected artifact state %s -> %s, got %s", expectedKey, expectedDigest, storedDigest)
	}
}

// TestSetArtifactState_InvalidImage tests if SetArtifactState fails for non-digest images.
func TestSetArtifactState_InvalidImage(t *testing.T) {

	state := State{
		Version:        "1.0",
		ArtifactStates: make(map[string]string),
	}
	artifactDir := "/artifacts/path"
	image := "invalid.repo/image:latest"

	err := state.SetArtifactState(artifactDir, image)
	if err == nil {
		t.Fatalf("Expected error for non-digest image, but got none")
	}
	expectedErr := "image invalid.repo/image:latest is not an image with digest"
	if err.Error() != expectedErr {
		t.Errorf("Expected error: %s, got: %s", expectedErr, err.Error())
	}
}

// TestGetArtifactState_Success tests if GetArtifactState retrieves a stored digest correctly.
func TestGetArtifactState_Success(t *testing.T) {
	state := State{
		Version: "1.0",
		ArtifactStates: map[string]string{
			"(/artifacts/path,valid.repo/image)": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}
	artifactDir := "/artifacts/path"
	image := "valid.repo/image"

	d, err := state.GetArtifactState(artifactDir, image)
	if err != nil {
		t.Fatalf("GetArtifactState failed: %v", err)
	}

	expectedDigest := digest.NewDigestFromEncoded("sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if *d != expectedDigest {
		t.Errorf("Expected digest %s, got %s", expectedDigest, *d)
	}
}

// TestGetArtifactState_NotFound tests if GetArtifactState returns an error when the key is missing.
func TestGetArtifactState_NotFound(t *testing.T) {
	state := State{
		Version:        "1.0",
		ArtifactStates: make(map[string]string),
	}
	artifactDir := "/artifacts/path"
	image := "valid.repo/image"

	_, err := state.GetArtifactState(artifactDir, image)
	if err == nil {
		t.Fatalf("Expected error for missing artifact state, but got none")
	}
	expectedErr := "artifact state not found for valid.repo/image"
	if err.Error() != expectedErr {
		t.Errorf("Expected error: %s, got: %s", expectedErr, err.Error())
	}
}
