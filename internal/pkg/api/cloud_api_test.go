package api

import (
	"bytes"
	"errors"
	"testing"

	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/internal/pkg/utils"
)

type ArtifactStorageStub struct {
	StoredArtifact artifact.Artifact
}

//goland:noinspection GoUnusedParameter
func (stor *ArtifactStorageStub) LoadArtifact(identifier string) (artifact.Artifact, error) {
	if stor.StoredArtifact != nil {
		return stor.StoredArtifact, nil
	}
	return nil, dorasErrors.ErrArtifactNotFound
}

//goland:noinspection GoUnusedParameter
func (stor *ArtifactStorageStub) StoreArtifact(a artifact.Artifact, identifier string) error {
	stor.StoredArtifact = a
	return nil
}

//goland:noinspection GoUnusedParameter,GoUnusedParameter
func (stor *ArtifactStorageStub) StoreDelta(d delta.ArtifactDelta, identifier string) error {
	panic("not implemented")
}

func (stor *ArtifactStorageStub) LoadDelta(string) (delta.ArtifactDelta, error) {
	panic("not implemented")
}

type AliasStub struct {
	Key   string
	Value string
}

func (a *AliasStub) AddAlias(alias, identifier string) error {
	if a.Key != alias {
		return dorasErrors.ErrAliasExists
	}
	a.Value = identifier
	a.Key = alias
	return nil
}

func (a *AliasStub) ResolveAlias(alias string) (string, error) {
	if alias == a.Key {
		return a.Value, nil
	}
	return "", dorasErrors.ErrAliasNotFound
}

func Test_createArtifact(t *testing.T) {
	stor := ArtifactStorageStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
	}
	artfct := artifact.RawBytesArtifact{Data: []byte("foo")}
	identifier, err := cloudAPI.createArtifact(&artfct)
	if err != nil {
		t.Fail()
	}
	if !bytes.Equal(stor.StoredArtifact.GetBytes(), artfct.Data) {
		t.Fail()
	}
	if utils.CalcSha256Hex([]byte("foo")) != identifier {
		t.Fail()
	}
}

func Test_createNamedArtifact(t *testing.T) {
	stor := ArtifactStorageStub{}
	aliaser := AliasStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &aliaser,
	}
	alias := "foo.bar"
	artfct := artifact.RawBytesArtifact{Data: []byte("foo")}
	identifier, err := cloudAPI.createNamedArtifact(&artfct, alias)
	if err != nil {
		return
	}
	if !bytes.Equal(stor.StoredArtifact.GetBytes(), artfct.Data) {
		t.Fatal("stored artifact does not match expected artifact")
	}
	if aliaser.Key != alias {
		t.Fatalf("did not store alias correctly, got: %s expected: %s", aliaser.Value, alias)
	}
	expectedIdentifier := utils.CalcSha256Hex([]byte("foo"))
	if expectedIdentifier != identifier {
		t.Fatalf("did not return correct identifier, got: %s expected: %s", aliaser.Value, identifier)
	}
	if expectedIdentifier != aliaser.Value {
		t.Fatalf("did not store correct alias mapping, got: %s expected: %s", aliaser.Value, identifier)
	}
}

func Test_createNamedArtifactNameCollision(t *testing.T) {
	stor := ArtifactStorageStub{}
	aliaser := AliasStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &aliaser,
	}
	alias := "bar"
	_, err := cloudAPI.createNamedArtifact(&artifact.RawBytesArtifact{}, alias)
	if err == nil {
		t.Fatalf("did not get an error when it was expected")
	}
	if !errors.Is(err, dorasErrors.ErrAliasExists) {
		t.Fatalf("did not get the appropriate error: %s", err)
	}
	if len(stor.StoredArtifact.GetBytes()) != 0 {
		t.Fatalf("stored artifact was not empty when it should have been")
	}
}

func Test_readArtifact(t *testing.T) {
	stor := ArtifactStorageStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
	}
	expected := []byte("bar")
	stor.StoredArtifact = &artifact.RawBytesArtifact{Data: expected}

	artfct, err := cloudAPI.readArtifact("foo")
	if err != nil {
		t.Fatalf("expected artifact, got %s", err)
	}
	got := artfct.GetBytes()
	if !bytes.Equal(got, expected) {
		t.Fatalf("artifact does not match expected artifact, got: %s expected: %s", got, expected)
	}
}
func Test_readArtifactArtifactNotFound(t *testing.T) {
	stor := ArtifactStorageStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
	}

	artfct, err := cloudAPI.readArtifact("foo")
	if err == nil {
		t.Fatalf("expected error, got %s", artfct)
	}
	if !errors.Is(err, dorasErrors.ErrArtifactNotFound) {
		t.Fatalf("did not get the appropriate error: %s", err)
	}
}

func Test_readNamedArtifact(t *testing.T) {
	stor := ArtifactStorageStub{}
	aliaser := AliasStub{
		Key:   "foo",
		Value: "bar",
	}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &aliaser,
	}
	expected := []byte("bar")
	stor.StoredArtifact = &artifact.RawBytesArtifact{Data: expected}

	artfct, err := cloudAPI.readNamedArtifact("foo")
	if err != nil {
		t.Fatalf("expected artifact, got %s", err)
	}
	got := artfct.GetBytes()
	if !bytes.Equal(got, expected) {
		t.Fatalf("artifact does not match expected artifact, got: %s expected: %s", got, expected)
	}
}

func Test_readNamedArtifactAliasNotFound(t *testing.T) {
	stor := ArtifactStorageStub{}
	aliaser := AliasStub{}
	cloudAPI := CloudAPI{
		artifactStorageProvider: &stor,
		aliasProvider:           &aliaser,
	}

	artfct, err := cloudAPI.readNamedArtifact("foo")
	if err == nil {
		t.Fatalf("expected error, got %s", artfct)
	}
	if !errors.Is(err, dorasErrors.ErrAliasNotFound) {
		t.Fatalf("did not get the appropriate error: %s", err)
	}
}
