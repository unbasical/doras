package updater

import (
	"context"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	gzip2 "github.com/unbasical/doras/internal/pkg/compression/gzip"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/internal/pkg/utils/testutils"
	"github.com/unbasical/doras/pkg/client/edgeapi"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"io"
	"oras.land/oras-go/v2"
	"os"
	"path"
	"testing"
)

func TestClient_PullAsync(t *testing.T) {
	pFrom := "../../../test/test-files/from.tar.gz"
	pTo := "../../../test/test-files/to.tar.gz"

	tempDir := t.TempDir()
	outDir := path.Join(tempDir, "out")
	_ = os.Mkdir(outDir, 0777)

	expectedDir := path.Join(tempDir, "expected")
	_ = os.Mkdir(expectedDir, 0777)
	err := tarutils.ExtractCompressedTar(expectedDir, "", pTo, nil, gzip2.NewDecompressor())
	if err != nil {
		t.Fatal(err)
	}
	expectedDirErr := path.Join(tempDir, "expected-err")
	_ = os.Mkdir(expectedDirErr, 0777)
	err = tarutils.ExtractCompressedTar(expectedDirErr, "", pFrom, nil, gzip2.NewDecompressor())
	if err != nil {
		t.Fatal(err)
	}
	expectedDirEmpty := path.Join(tempDir, "expected-empty")
	_ = os.Mkdir(expectedDirEmpty, 0777)

	internalDir := path.Join(tempDir, "internal")
	_ = os.Mkdir(internalDir, 0777)

	ctx := context.Background()
	s, err := testutils.StorageFromFiles(ctx, t.TempDir(), []testutils.FileDescription{
		{Name: "archive", Data: fileutils.ReadOrPanic(pFrom), Tag: "v1", NeedsUnpack: true},
		{Name: "archive", Data: fileutils.ReadOrPanic(pTo), Tag: "v2", NeedsUnpack: true},
		{Name: "delta.patch.tardiff", Data: fileutils.ReadOrPanic("../../../test/test-files/delta.patch.tardiff"), Tag: "delta", NeedsUnpack: false, MediaType: "application/tardiff"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repoName := "registry.example.org/foo"
	currentDescriptor := func() ocispec.Descriptor {
		d, err := s.Resolve(ctx, "v1")
		if err != nil {
			t.Fatal()
		}
		return d
	}()
	targetDescriptor := func() ocispec.Descriptor {
		d, err := s.Resolve(ctx, "v2")
		if err != nil {
			t.Fatal()
		}
		return d
	}()
	deltaDescriptor := func() ocispec.Descriptor {
		d, err := s.Resolve(ctx, "delta")
		if err != nil {
			t.Fatal()
		}
		return d
	}()
	descriptorsToPaths := map[digest.Digest]string{
		currentDescriptor.Digest: pFrom,
		targetDescriptor.Digest:  pTo,
	}
	targetImage := fmt.Sprintf("%s@%s", repoName, targetDescriptor.Digest.String())
	deltaImage := fmt.Sprintf("%s@%s", repoName, deltaDescriptor.Digest.String())
	type initialState struct {
		version *ocispec.Descriptor
	}
	type fields struct {
		opts       clientOpts
		edgeClient edgeapi.DeltaApiClient
		reg        fetcher.ArtifactLoader
	}
	type args struct {
		target string
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		expectedDir    string
		expectedDigest *digest.Digest
		wantExists     bool
		wantErr        bool
		initialState
	}{
		{
			name: "success (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s})},
			args: args{
				target: targetImage,
			},
			wantExists:     true,
			wantErr:        false,
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: &currentDescriptor,
			},
		},
		{
			name: "success (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s})},
			args: args{
				target: targetImage,
			},
			wantExists:     true,
			wantErr:        false,
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: nil,
			},
		},
		{
			name: "error (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}),
			},
			args: args{
				target: targetImage,
			},
			wantExists:     false,
			wantErr:        true,
			expectedDir:    expectedDirErr,
			expectedDigest: &currentDescriptor.Digest,
			initialState: initialState{
				version: &currentDescriptor,
			},
		},
		{
			name: "resolve error (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s})},
			args: args{
				target: "registry.example.org/foo:bar",
			},
			wantExists:     false,
			wantErr:        true,
			expectedDir:    expectedDirEmpty,
			expectedDigest: nil,
			initialState: initialState{
				version: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fileutils.CleanDirectory(outDir)
			if err != nil {
				t.Fatal(err)
			}
			err = fileutils.CleanDirectory(internalDir)
			if err != nil {
				t.Fatal(err)
			}
			if tt.initialState.version != nil {
				p := descriptorsToPaths[tt.initialState.version.Digest]
				err := tarutils.ExtractCompressedTar(outDir, "", p, nil, gzip2.NewDecompressor())
				if err != nil {
					t.Fatal(err)
				}
			}
			s, err := statemanager.New(updaterstate.State{
				Version: "1",
				ArtifactStates: func() map[string]string {
					if tt.initialState.version == nil {
						return map[string]string{}
					}
					return map[string]string{
						fmt.Sprintf("(%s,%s)", outDir, repoName): tt.initialState.version.Digest.Encoded(),
					}
				}(),
			}, path.Join(internalDir, "state.json"))
			funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed init state")
			c := &Client{
				opts:       tt.fields.opts,
				edgeClient: tt.fields.edgeClient,
				reg:        tt.fields.reg,
				state:      s,
			}
			gotExists, err := c.PullAsync(tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("PullAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotExists != tt.wantExists {
				t.Errorf("PullAsync() gotExists = %v, want %v", gotExists, tt.wantExists)
			}
			eq, err := fileutils.CompareDirectories(outDir, tt.expectedDir)
			if err != nil || !eq {
				t.Fatalf("directories not equal: %q", err)
			}
			st, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			appliedVersion, err := st.GetArtifactState(outDir, repoName)
			if err != nil && tt.expectedDigest != nil {
				t.Fatal(err)
			}
			if tt.expectedDigest != nil && *appliedVersion != *tt.expectedDigest {
				t.Errorf("state does not contain correct version: got: %v ,expected: %v", *appliedVersion, tt.expectedDigest)
			}
		})
	}
}

type mockStorageSource struct {
	s oras.ReadOnlyTarget
}

func (m *mockStorageSource) GetTarget(_ string) (oras.ReadOnlyTarget, error) {
	return m.s, nil
}

type mockApiClient struct {
	f func() (res *apicommon.ReadDeltaResponse, exists bool, err error)
}

func (m *mockApiClient) ReadDeltaAsync(_, _ string, _ []string) (res *apicommon.ReadDeltaResponse, exists bool, err error) {
	return m.f()
}

func (m *mockApiClient) ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, error) {
	panic("not implemented")
}

func (m *mockApiClient) ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*ocispec.Descriptor, string, io.ReadCloser, error) {
	panic("not implemented")
}
