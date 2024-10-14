package api

import (
	"bytes"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/differ"
	error2 "github.com/unbasical/doras-server/internal/pkg/error"
	"testing"
)

type MapArtifactStorage struct {
	Artifacts map[string]artifact.Artifact
	Deltas    map[string]delta.ArtifactDelta
}

func (store *MapArtifactStorage) LoadArtifact(identifier string) (artifact.Artifact, error) {
	artfct, ok := store.Artifacts[identifier]
	if !ok {
		return nil, error2.DorasArtifactNotFoundError
	}
	return artfct, nil
}

func (store *MapArtifactStorage) StoreArtifact(artifact artifact.Artifact, identifier string) error {
	store.Artifacts[identifier] = artifact
	return nil
}
func (store *MapArtifactStorage) StoreDelta(d delta.ArtifactDelta, identifier string) error {
	log.Debug("before: ", store.Deltas)
	store.Deltas[identifier] = d
	log.Debug("after: ", store.Deltas)
	return nil
}

func (store *MapArtifactStorage) LoadDelta(identifier string) (delta.ArtifactDelta, error) {
	dt, ok := store.Deltas[identifier]
	if !ok {
		return nil, error2.DorasDeltaNotFoundError
	}
	return dt, nil
}

func TestEdgeAPI_createDelta(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}

	a1Data := []byte("foo")
	a1 := artifact.RawBytesArtifact{Data: a1Data}
	a2Data := []byte("bar")
	a2 := artifact.RawBytesArtifact{Data: a2Data}

	diffExpected := differ.Bsdiff{}.CreateDiff(&a1, &a2)

	stor.Artifacts["a1"] = &a1
	stor.Artifacts["a2"] = &a2

	deltaIdentifier, err := edgeAPI.createDelta("a1", "a2", "bsdiff")
	if err != nil {
		t.Fatal(err)
	}
	// internally stores the delta with the algorithm as the extension
	diffGot, ok := stor.Deltas[deltaIdentifier+".bsdiff"]
	if !ok {
		t.Fatalf("Delta %s not found in map %s", deltaIdentifier, stor.Deltas[deltaIdentifier])
	}
	diffGotRaw, _ := diffGot.GetBytes()
	if !bytes.Equal(diffExpected, diffGotRaw) {
		t.Fatalf("storage did not contain the expected diff: expected %s got: %s", diffExpected, diffGotRaw)
	}
}

func TestEdgeAPI_createDeltaErrors(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}

	deltaIdentifier, err := edgeAPI.createDelta("a1", "a2", "bsdiff")
	if err == nil {
		t.Fatalf("expected error, got %s", deltaIdentifier)
	}
	if !errors.Is(err, error2.DorasArtifactNotFoundError) {
		t.Fatalf("did not return the appropriate error, got: %s", err)
	}
}

func TestEdgeAPI_readDelta(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}
	expected := []byte("foo")
	stor.Deltas["delta.bsdiff"] = &delta.RawDiff{Data: expected}
	reader, n, err := edgeAPI.readDelta("delta", "bsdiff")
	if err != nil {
		return
	}
	got := make([]byte, n)
	_, err = reader.Read(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, got) {
		t.Fatalf("did not return the expected diff: expected %s got: %s", expected, got)
	}
}

func TestEdgeAPI_readDeltaError(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}

	reader, _, err := edgeAPI.readDelta("delta", "bsdiff")
	if err == nil {
		t.Fatalf("expected error, got %s", reader)
	}
	if !errors.Is(err, error2.DorasDeltaNotFoundError) {
		t.Fatalf("did not return the appropriate error, got: %s", err)
	}
}

func TestEdgeAPI_readFull(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}
	expected := []byte("foo")
	stor.Artifacts["bar"] = &artifact.RawBytesArtifact{Data: expected}
	reader, n, err := edgeAPI.readFull("bar")
	if err != nil {
		return
	}
	got := make([]byte, n)
	_, err = reader.Read(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, got) {
		t.Fatalf("did not return the expected diff: expected %s got: %s", expected, got)
	}
}

func TestEdgeAPI_readFullError(t *testing.T) {
	stor := MapArtifactStorage{
		Artifacts: make(map[string]artifact.Artifact),
		Deltas:    make(map[string]delta.ArtifactDelta),
	}
	edgeAPI := EdgeAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &AliasStub{},
	}
	reader, _, err := edgeAPI.readFull("bar")
	if err == nil {
		t.Fatalf("expected error, got %s", reader)
	}
	if errors.Is(err, error2.DorasDeltaNotFoundError) {
		t.Fatalf("did not return the appropriate error, got: %s", err)
	}
}
