package updaterstate

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	"golang.org/x/mod/sumdb/dirhash"
	"testing"
)

// TestSetArtifactState_Success tests if SetArtifactState correctly stores a digest.
func TestSetArtifactState_Success(t *testing.T) {

	state := State{
		Version:        "2",
		ArtifactStates: make(map[string]ArtifactState),
	}
	artifactDir := "/artifacts/path"
	image := "valid.repo/image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	dirHash, err := dirhash.HashDir(t.TempDir(), "", dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	dirHashDigest := digest.Digest(dirHash)
	err = state.SetArtifactState(artifactDir, image, dirHashDigest)
	if err != nil {
		t.Fatalf("SetArtifactState failed: %v", err)
	}

	expectedKey := "(%s,%s)"
	expectedKey = fmt.Sprintf(expectedKey, artifactDir, "valid.repo/image")
	expectedDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	expectedArtifactState := ArtifactState{
		ImageDigest:     digest.NewDigestFromHex("sha256", expectedDigest),
		DirectoryDigest: dirHashDigest,
	}
	if storedState, exists := state.ArtifactStates[expectedKey]; !exists || storedState != expectedArtifactState {
		t.Errorf("Expected artifact state %s -> %s, got %s", expectedKey, expectedDigest, storedState)
	}
}

// TestSetArtifactState_InvalidImage tests if SetArtifactState fails for non-digest images.
func TestSetArtifactState_InvalidImage(t *testing.T) {
	state := State{
		Version:        "2",
		ArtifactStates: make(map[string]ArtifactState),
	}
	artifactDir := "/artifacts/path"
	image := "invalid.repo/image:latest"
	dirHash, err := dirhash.HashDir(t.TempDir(), "", dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	dirHashDigest := digest.Digest(dirHash)

	err = state.SetArtifactState(artifactDir, image, dirHashDigest)
	if err == nil {
		t.Fatalf("Expected error for non-digest image, but got none")
	}
}

// TestGetArtifactState_Success tests if GetArtifactState retrieves a stored digest correctly.
func TestGetArtifactState_Success(t *testing.T) {
	dirHash, err := dirhash.HashDir(t.TempDir(), "", dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}
	dirHashDigest := digest.Digest(dirHash)
	artifactDir := "/artifacts/path"
	expectedKey := "(%s,%s)"
	expectedDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	expectedKey = fmt.Sprintf(expectedKey, artifactDir, "valid.repo/image")
	expectedArtifactState := ArtifactState{
		ImageDigest:     digest.NewDigestFromHex("sha256", expectedDigest),
		DirectoryDigest: dirHashDigest,
	}
	state := State{
		Version: "2",
		ArtifactStates: map[string]ArtifactState{
			expectedKey: expectedArtifactState,
		},
	}
	s, err := state.GetArtifactState(artifactDir, "valid.repo/image")
	if err != nil {
		t.Fatalf("GetArtifactState failed: %v", err)
	}
	if s != expectedArtifactState {
		t.Errorf("Expected artifact state %s -> %s, got %s", expectedKey, expectedArtifactState, s)
	}
}

// TestGetArtifactState_NotFound tests if GetArtifactState returns an error when the key is missing.
func TestGetArtifactState_NotFound(t *testing.T) {
	artifactDir := "/artifacts/path"
	state := State{
		Version:        "2",
		ArtifactStates: map[string]ArtifactState{},
	}
	image := "valid.repo/image@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	_, err := state.GetArtifactState(artifactDir, image)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
