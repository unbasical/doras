package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	gzip2 "github.com/unbasical/doras/internal/pkg/compression/gzip"
	bsdiff2 "github.com/unbasical/doras/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras/internal/pkg/utils/fileutils"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/tarutils"
	"github.com/unbasical/doras/internal/pkg/utils/testutils"
	"github.com/unbasical/doras/pkg/client/edgeapi"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/inspector"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"github.com/unbasical/doras/pkg/client/updater/validator"
	"golang.org/x/mod/sumdb/dirhash"
	"oras.land/oras-go/v2"
)

func TestClient_PullAsyncTardiff(t *testing.T) {
	pFromTardiff := "../../../test/test-files/from.tar.gz"
	pToTardiff := "../../../test/test-files/to.tar.gz"
	tempDir := t.TempDir()
	outDir := path.Join(tempDir, "out")
	_ = os.Mkdir(outDir, 0777)

	expectedDir := path.Join(tempDir, "expected")
	_ = os.Mkdir(expectedDir, 0777)
	err := tarutils.ExtractCompressedTar(expectedDir, "", pToTardiff, nil, gzip2.NewDecompressor())
	if err != nil {
		t.Fatal(err)
	}
	expectedDirErr := path.Join(tempDir, "expected-err")
	_ = os.Mkdir(expectedDirErr, 0777)
	err = tarutils.ExtractCompressedTar(expectedDirErr, "", pFromTardiff, nil, gzip2.NewDecompressor())
	if err != nil {
		t.Fatal(err)
	}
	expectedDirEmpty := path.Join(tempDir, "expected-empty")
	_ = os.Mkdir(expectedDirEmpty, 0777)

	internalDir := path.Join(tempDir, "internal")
	_ = os.Mkdir(internalDir, 0777)

	ctx := context.Background()
	s, err := testutils.StorageFromFiles(ctx, t.TempDir(), []testutils.FileDescription{
		{Name: "archive", Data: fileutils.ReadOrPanic(pFromTardiff), Tag: "v1", NeedsUnpack: true},
		{Name: "archive", Data: fileutils.ReadOrPanic(pToTardiff), Tag: "v2", NeedsUnpack: true},
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
		currentDescriptor.Digest: pFromTardiff,
		targetDescriptor.Digest:  pToTardiff,
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
	statsDir := path.Join(t.TempDir(), "stats-dir")
	tests := []struct {
		name           string
		fields         fields
		args           args
		expectedDir    string
		expectedDigest *digest.Digest
		wantExists     bool
		wantErr        bool
		wantSize       *struct {
			n   uint64
			dir string
		}
		initialState
	}{
		{
			name: "success (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
			name: "success (initialized, but delta not possible)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, apicommon.ErrImagesIncompatible
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
			name: "success (images are identical)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, apicommon.ErrImagesIdentical
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     true,
			wantErr:        false,
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: &targetDescriptor,
			},
		},
		{
			name: "error (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil),
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
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
		{
			name: "error size limit (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 1},
				}, nil)},
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
			name: "error size limit (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
						Validators: []validator.ManifestValidator{
							validator.SizeLimitedValidator{Limit: 1},
						},
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 1},
				}, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     false,
			wantErr:        true,
			expectedDir:    expectedDirEmpty,
			expectedDigest: nil,
			initialState: initialState{
				version: nil,
			},
		},
		{
			name: "success size limit (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 0xffff},
				}, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     true,
			wantErr:        false,
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: &targetDescriptor,
			},
		},
		{
			name: "success size limit (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
						Validators: []validator.ManifestValidator{
							validator.SizeLimitedValidator{Limit: 1},
						},
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 0xffff},
				}, nil)},
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
			name: "success volume limit (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
						Validators: []validator.ManifestValidator{
							validator.SizeLimitedValidator{Limit: 10000},
						},
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.VolumeLimitValidator{
						Limit:    10,
						Period:   funcutils.Unwrap(time.ParseDuration("24h")),
						StatsDir: path.Join(t.TempDir(), "volume-limit-uninitialized"),
					},
				}, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     false,
			wantErr:        true,
			expectedDir:    expectedDirEmpty,
			expectedDigest: nil,
			initialState: initialState{
				version: nil,
			},
		},
		{
			name: "error volume limit (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
						Validators: []validator.ManifestValidator{
							validator.SizeLimitedValidator{Limit: 1},
						},
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.VolumeLimitValidator{
						Limit:    10,
						Period:   funcutils.Unwrap(time.ParseDuration("24h")),
						StatsDir: path.Join(t.TempDir(), "volume-limit-uninitialized"),
					},
				}, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     false,
			wantErr:        true,
			expectedDir:    expectedDirEmpty,
			expectedDigest: nil,
			initialState: initialState{
				version: nil,
			},
		},
		{
			name: "error volume limit (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.VolumeLimitValidator{
						Limit:  0xffff,
						Period: funcutils.Unwrap(time.ParseDuration("24h")),
						StatsDir: func() string {
							statsDir := path.Join(t.TempDir(), "volume-limit-uninitialized")
							err := validator.WriteUintToFile(statsDir, 0xffff-1)
							if err != nil {
								t.Fatal(err)
							}
							return statsDir
						}(),
					},
				}, nil)},
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
			name: "download stats (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s},
					nil,
					[]inspector.ArtifactInspector{
						inspector.NewDownloadStatsObserver(path.Join(statsDir, "uninitialized")),
					},
				)},
			args: args{
				target: targetImage,
			},
			wantExists: true,
			wantErr:    false,
			wantSize: &struct {
				n   uint64
				dir string
			}{n: 359, dir: path.Join(statsDir, "uninitialized")},
			expectedDir:    expectedDir,
			expectedDigest: nil,
			initialState: initialState{
				version: nil,
			},
		},
		{
			name: "download stats (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil,
					[]inspector.ArtifactInspector{
						inspector.NewDownloadStatsObserver(path.Join(statsDir, "initialized")),
					})},
			args: args{
				target: targetImage,
			},
			wantExists: true,
			wantErr:    false,
			wantSize: &struct {
				n   uint64
				dir string
			}{n: 338, dir: path.Join(statsDir, "initialized")},
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: &currentDescriptor,
			},
		},
	}
	for _, tt := range tests {
		for _, keepOldDir := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s_%v", tt.name, keepOldDir), func(t *testing.T) {
				err := fileutils.CleanDirectory(outDir)
				if err != nil {
					t.Fatal(err)
				}
				err = fileutils.CleanDirectory(internalDir)
				if err != nil {
					t.Fatal(err)
				}
				if tt.version != nil {
					p := descriptorsToPaths[tt.version.Digest]
					err := tarutils.ExtractCompressedTar(outDir, "", p, nil, gzip2.NewDecompressor())
					if err != nil {
						t.Fatal(err)
					}
				}
				dirHash, err := dirhash.HashDir(outDir, "", dirhash.Hash1)
				if err != nil {
					t.Fatal(err)
				}
				dirHashDigest := digest.Digest(dirHash)
				s, err := statemanager.New(updaterstate.State{
					Version: "2",
					ArtifactStates: func() map[string]updaterstate.ArtifactState {
						if tt.version == nil {
							return map[string]updaterstate.ArtifactState{}
						}
						return map[string]updaterstate.ArtifactState{
							fmt.Sprintf("(%s,%s)", outDir, repoName): updaterstate.ArtifactState{
								ImageDigest:     tt.version.Digest,
								DirectoryDigest: dirHashDigest,
							},
						}
					}(),
				}, path.Join(internalDir, "state.json"))
				funcutils.PanicOrLogOnErr(funcutils.IdentityFunc(err), true, "failed init state")
				tt.fields.opts.KeepOldDir = keepOldDir
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
				if err != nil {
					t.Fatal(err)
				}
				if !eq {
					t.Fatal("directories not equal")
				}
				st, err := s.Load()
				if err != nil {
					t.Fatal(err)
				}
				artifactState, err := st.GetArtifactState(outDir, repoName)
				if err != nil && tt.expectedDigest != nil {
					t.Fatal(err)
				}
				if tt.expectedDigest != nil && artifactState.ImageDigest != *tt.expectedDigest {
					t.Errorf("state does not contain correct version: got: %v ,expected: %v", artifactState.ImageDigest, tt.expectedDigest)
				}
				if tt.version != nil {
					expectedDirHash, err := dirhash.HashDir(outDir, "", dirhash.Hash1)
					if err != nil {
						t.Fatal(err)
					}
					expectedDirHashDigest := digest.Digest(expectedDirHash)
					if artifactState.DirectoryDigest != expectedDirHashDigest {
						t.Errorf("output directory does not have the expected dirhash epxected %v, got %v", expectedDirHashDigest, artifactState.DirectoryDigest)
					}
				}
				if tt.wantSize != nil {
					stats, err := validator.SumUpDownloadStats(tt.wantSize.dir, funcutils.Unwrap(time.ParseDuration("1h")))
					if err != nil {
						t.Error(err)
					}
					if stats != tt.wantSize.n {
						t.Errorf("expected to have downloaded %d bytes, got %d", tt.wantSize.n, stats)
					}
				}
			})
		}
	}
}

func TestClient_PullAsyncBsdiff(t *testing.T) {
	from := "hello"
	to := "hello world"
	diff, err := bsdiff2.NewDiffer().Diff(strings.NewReader(from), strings.NewReader(to))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = diff.Close()
	}()
	diffBytes, err := io.ReadAll(diff)
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	outDir := path.Join(tempDir, "out")
	_ = os.Mkdir(outDir, 0777)

	// patched
	expectedDir := path.Join(tempDir, "expected")
	_ = os.Mkdir(expectedDir, 0777)
	_ = os.WriteFile(path.Join(expectedDir, "artifact"), []byte(to), 0777)

	// no change
	expectedDirErr := path.Join(tempDir, "expected-err")
	_ = os.Mkdir(expectedDirErr, 0777)
	_ = os.WriteFile(path.Join(expectedDirErr, "artifact"), []byte(from), 0777)

	// uninitialized
	expectedDirEmpty := path.Join(tempDir, "expected-empty")
	_ = os.Mkdir(expectedDirEmpty, 0777)

	internalDir := path.Join(tempDir, "internal")
	_ = os.Mkdir(internalDir, 0777)

	ctx := context.Background()
	s, err := testutils.StorageFromFiles(ctx, t.TempDir(), []testutils.FileDescription{
		{Name: "artifact", Data: []byte(from), Tag: "v1"},
		{Name: "artifact", Data: []byte(to), Tag: "v2"},
		{Name: "delta.patch.bsdiff", Data: diffBytes, Tag: "delta", NeedsUnpack: false, MediaType: "application/bsdiff"},
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
	descriptorsToData := map[digest.Digest]string{
		currentDescriptor.Digest: from,
		targetDescriptor.Digest:  to,
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
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
			name: "success (initialized, but delta not possible)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, apicommon.ErrImagesIncompatible
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
			name: "success (images are identical)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, apicommon.ErrImagesIdentical
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
			args: args{
				target: targetImage,
			},
			wantExists:     true,
			wantErr:        false,
			expectedDir:    expectedDir,
			expectedDigest: &targetDescriptor.Digest,
			initialState: initialState{
				version: &targetDescriptor,
			},
		},
		{
			name: "error (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil),
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
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					return nil, false, errors.New("some error")
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, nil, nil)},
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
		{
			name: "error size limit (initialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:      outDir,
						InternalDirectory:    internalDir,
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 1},
				}, nil)},
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
			name: "error size limit (uninitialized)",
			fields: fields{
				opts: func() clientOpts {
					return clientOpts{
						OutputDirectory:   outDir,
						InternalDirectory: internalDir,
						Validators: []validator.ManifestValidator{
							validator.SizeLimitedValidator{Limit: 1},
						},
						OutputDirPermissions: 0755,
					}
				}(),
				edgeClient: &mockApiClient{f: func() (res *apicommon.ReadDeltaResponse, exists bool, err error) {
					retval := apicommon.ReadDeltaResponse{
						TargetImage: targetImage,
						DeltaImage:  deltaImage,
					}
					return &retval, true, nil
				}},
				reg: fetcher.NewArtifactLoader(t.TempDir(), &mockStorageSource{s: s}, []validator.ManifestValidator{
					validator.SizeLimitedValidator{Limit: 1},
				}, nil)},
			args: args{
				target: targetImage,
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
			if tt.version != nil {
				data := descriptorsToData[tt.version.Digest]
				_ = os.Remove(path.Join(outDir, "artifact"))
				err := os.WriteFile(path.Join(outDir, "artifact"), []byte(data), 0777)
				if err != nil {
					t.Fatal(err)
				}
			}
			dirHash, err := dirhash.HashDir(outDir, "", dirhash.Hash1)
			if err != nil {
				t.Fatal(err)
			}
			dirHashDigest := digest.Digest(dirHash)
			s, err := statemanager.New(updaterstate.State{
				Version: "2",
				ArtifactStates: func() map[string]updaterstate.ArtifactState {
					if tt.version == nil {
						return map[string]updaterstate.ArtifactState{}
					}
					return map[string]updaterstate.ArtifactState{
						fmt.Sprintf("(%s,%s)", outDir, repoName): updaterstate.ArtifactState{
							ImageDigest:     tt.version.Digest,
							DirectoryDigest: dirHashDigest,
						},
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
			if err != nil {
				t.Fatal(err)
			}
			if !eq {
				t.Fatal("directories not equal")
			}
			st, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			appliedVersion, err := st.GetArtifactState(outDir, repoName)
			if err != nil && tt.expectedDigest != nil {
				t.Fatal(err)
			}

			if tt.expectedDigest != nil && appliedVersion.ImageDigest != *tt.expectedDigest {
				t.Errorf("state does not contain correct version: got: %v ,expected: %v", appliedVersion.ImageDigest, tt.expectedDigest)
			}
			if tt.version != nil {
				expectedDirHash, err := dirhash.HashDir(outDir, "", dirhash.Hash1)
				if err != nil {
					t.Fatal(err)
				}
				expectedDirHashDigest := digest.Digest(expectedDirHash)
				if appliedVersion.DirectoryDigest != expectedDirHashDigest {
					t.Errorf("output directory does not have the expected dirhash epxected %v, got %v", expectedDirHashDigest, appliedVersion.DirectoryDigest)
				}
			}
		})
	}
}

func TestClient_DetectAndCleanOldStateVersion(t *testing.T) {
	tempDir, err := os.MkdirTemp(t.TempDir(), "doras-state-*")
	if err != nil {
		t.Fatal(err)
	}
	// this file should be automatically deleted by the constructor due to being version "1"
	err = os.WriteFile(path.Join(tempDir, "doras-state.json"), []byte(`{"version":"1"}`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	// this file should be automatically deleted by the constructor due to being version "1"
	err = os.WriteFile(path.Join(tempDir, "dummy"), []byte(""), 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient(WithInternalDirectory(tempDir))
	if err != nil {
		t.Fatal(err)
	}
	stateFile, err := os.ReadFile(path.Join(tempDir, "doras-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := struct {
		Version string `json:"version"`
	}{}
	err = json.Unmarshal(stateFile, &s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != "2" {
		t.Errorf("state file does not contain expected version: got %v, expected: %v", s.Version, "2")
	}
	if _, err := os.Stat(path.Join(tempDir, "dummy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected dummy file to be removed")
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
