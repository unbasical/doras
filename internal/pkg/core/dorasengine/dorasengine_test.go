package dorasengine

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	auth2 "github.com/unbasical/doras-server/internal/pkg/auth"

	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	deltadelegate "github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras-server/internal/pkg/delegates/registry"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/testutils"
	"github.com/unbasical/doras-server/pkg/constants"
	"oras.land/oras-go/v2"
)

type testRegistryDelegate struct {
	storage      oras.Target
	ctx          context.Context
	expectedAuth string
}

func (t *testRegistryDelegate) Resolve(image string, expectDigest bool, creds auth.CredentialFunc) (oras.ReadOnlyTarget, string, v1.Descriptor, error) {
	url, err := ociutils.ParseOciUrl(image)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	if t.expectedAuth != "" {
		if creds == nil {
			return nil, "", v1.Descriptor{}, fmt.Errorf("auth failure")
		}
		hostport := strings.TrimSuffix(fmt.Sprintf("%s:%s", url.Host, url.Port()), ":")
		c, err := creds(context.Background(), hostport)
		if err != nil {
			return nil, "", v1.Descriptor{}, err
		}
		if t.expectedAuth != c.AccessToken {
			return nil, "", v1.Descriptor{}, fmt.Errorf("auth failure")
		}
	}

	repo, tag, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	if expectDigest && !isDigest {
		return nil, "", v1.Descriptor{}, errors.New("digest expected")
	}
	if isDigest {
		tag = strings.TrimPrefix(tag, "@")
	}
	d, err := t.storage.Resolve(t.ctx, tag)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	image = fmt.Sprintf("%s@%s", repo, d.Digest.String())
	return t.storage, image, d, nil
}

func (t *testRegistryDelegate) LoadManifest(target v1.Descriptor, source oras.ReadOnlyTarget) (v1.Manifest, error) {
	rc, err := source.Fetch(t.ctx, target)
	if err != nil {
		return v1.Manifest{}, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close reader")
	mf, err := ociutils.ParseManifestJSON(rc)
	if err != nil {
		return v1.Manifest{}, err
	}
	return *mf, nil
}

func (t *testRegistryDelegate) LoadArtifact(mf v1.Manifest, source oras.ReadOnlyTarget) (io.ReadCloser, error) {
	return source.Fetch(t.ctx, mf.Layers[0])
}

func (t *testRegistryDelegate) PushDelta(ctx context.Context, image string, manifOpts registrydelegate.DeltaManifestOptions, content io.ReadCloser) error {
	_, tag, _, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return err
	}

	tempDir := os.TempDir()
	fp, err := os.CreateTemp(tempDir, "delta_*")
	if err != nil {
		return err
	}
	defer func() {
		if err := errors.Join(fp.Close(), os.Remove(fp.Name())); err != nil {
			logrus.WithError(err).Error("failed temp file clean up")
		}
	}()

	// we need to write it to the disk because we cannot push without knowing the hash of the data
	// hash file while writing it to the disk
	hasher := sha256.New()
	teeReader := io.NopCloser(io.TeeReader(content, hasher))
	n, err := io.Copy(fp, teeReader)
	if err != nil {
		return err
	}
	deltaDescriptor := v1.Descriptor{
		MediaType: manifOpts.GetMediaType(),
		Digest:    digest.NewDigest("sha256", hasher),
		Size:      n,
		URLs:      nil,
		Annotations: map[string]string{
			"org.opencontainers.image.title": "delta" + manifOpts.GetFileExt(),
		},
	}
	_, err = fp.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = t.storage.Push(t.ctx, deltaDescriptor, fp)
	if err != nil {
		return err
	}
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{deltaDescriptor},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom: manifOpts.From,
			constants.DorasAnnotationTo:   manifOpts.To,
		},
	}
	mfDescriptor, err := oras.PackManifest(t.ctx, t.storage, oras.PackManifestVersion1_1, "application/vnd.example+type", opts)
	if err != nil {
		return err
	}
	err = t.storage.Tag(t.ctx, mfDescriptor, tag)
	if err != nil {
		return err
	}
	logrus.Infof("created delta at %s with (tag/digest) (%s/%s)", image, tag, mfDescriptor.Digest.Encoded())
	return nil
}

func (t *testRegistryDelegate) PushDummy(image string, manifOpts registrydelegate.DeltaManifestOptions) error {
	_, tag, _, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return err
	}

	ctx := context.Background()
	// Dummy manifests use the empty descriptor and set a value in the annotations to indicate a dummy.
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{v1.DescriptorEmptyJSON},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom:    manifOpts.From,
			constants.DorasAnnotationTo:      manifOpts.To,
			constants.DorasAnnotationIsDummy: "true",
		},
	}
	mfDescriptor, err := oras.PackManifest(ctx, t.storage, oras.PackManifestVersion1_1, "application/vnd.example+type", opts)
	if err != nil {
		return fmt.Errorf("failed to pack manifest: %v", err)
	}
	err = t.storage.Tag(ctx, mfDescriptor, tag)
	if err != nil {
		return fmt.Errorf("failed to tag manifest: %v", err)
	}
	logrus.Infof("created dummy at %s", image)
	return nil
}

type testAPIDelegate struct {
	token              string
	fromImage          string
	toImage            string
	acceptedAlgorithms []string
	lastErr            error
	lastErrMsg         string
	response           apicommon.ReadDeltaResponse
	hasHandledCallback bool
}

func (t *testAPIDelegate) RequestContext() (context.Context, error) {
	return context.Background(), nil
}

func (t *testAPIDelegate) ExtractClientAuth() (auth2.RegistryAuth, error) {
	if t.token != "" {
		return auth2.NewClientAuthFromToken(t.token), nil
	}
	return nil, errors.New("no token provided")
}

func (t *testAPIDelegate) ExtractParams() (fromImage, toImage string, acceptedAlgorithms []string, err error) {
	return t.fromImage, t.toImage, t.acceptedAlgorithms, nil
}

func (t *testAPIDelegate) HandleError(err error, msg string) {
	t.lastErrMsg = msg
	t.lastErr = err
	t.hasHandledCallback = true
}

func (t *testAPIDelegate) HandleSuccess(response any) {
	deltaResponse := response.(apicommon.ReadDeltaResponse)
	t.response = deltaResponse
	t.hasHandledCallback = true
}

func (t *testAPIDelegate) HandleAccepted() {

}

func Test_readDelta(t *testing.T) {
	ctx := context.Background()
	files := []testutils.FileDescription{
		{Name: "foobar", Data: []byte("foo"), Tag: "v1", NeedsUnpack: false},
		{Name: "foobar", Data: []byte("bar"), Tag: "v2", NeedsUnpack: false},
	}
	storage, err := testutils.StorageFromFiles(ctx, t.TempDir(), files)
	if err != nil {
		t.Fatal(err)
	}
	storageTarget, ok := (storage).(oras.Target)
	if !ok {
		t.Fatal("expected oras.Target")
	}
	registryMock := &testRegistryDelegate{
		storage: storageTarget,
	}
	_, image1, d, err := registryMock.Resolve("registry.example.org/foobar:v1", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	image1 = strings.ReplaceAll(image1, ":v1", "@"+d.Digest.String())

	_, image2, _, err := registryMock.Resolve("registry.example.org/foobar:v2", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	delegate := deltadelegate.NewDeltaDelegate()

	type args struct {
		registry    registrydelegate.RegistryDelegate
		delegate    deltadelegate.DeltaDelegate
		apiDelegate testAPIDelegate
		expectErr   bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					fromImage:          image1,
					toImage:            image2,
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: false,
			},
		},
		{
			name: "success to tag",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					fromImage:          image1,
					toImage:            "registry.example.org/foobar:v2",
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: false,
			},
		},
		{
			name: "reject non-digest",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					fromImage:          "registry.example.org/foobar:v1",
					toImage:            "registry.example.org/foobar:v2",
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: true,
			},
		},
		{
			name: "reject invalid repository",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					fromImage:          strings.Replace(image1, "foobar", "barfoo", 1),
					toImage:            image2,
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The purpose of this loop is to make sure the request has executed fully.
			// This is necessary due to the asynchronous nature of the readDelta function,
			// which spawns a go routine.
			wg := &sync.WaitGroup{}
			ctx := context.WithValue(context.Background(), "wg", wg)
			for {
				readDelta(ctx, tt.args.registry, tt.args.delegate, &tt.args.apiDelegate)
				if tt.args.apiDelegate.hasHandledCallback {
					break
				}
			}
			err = tt.args.apiDelegate.lastErr
			if (err != nil) != tt.args.expectErr {
				t.Fatalf("readDelta() error = %v, wantErr %v", err, tt.args.expectErr)
				return
			}
			response := tt.args.apiDelegate.response
			fmt.Printf("%v\n", response)
		})
	}
}

func Test_readDelta_Token(t *testing.T) {
	ctx := context.Background()
	files := []testutils.FileDescription{
		{Name: "foobar", Data: []byte("foo"), Tag: "v1", NeedsUnpack: false},
		{Name: "foobar", Data: []byte("bar"), Tag: "v2", NeedsUnpack: false},
	}
	storage, err := testutils.StorageFromFiles(ctx, t.TempDir(), files)
	if err != nil {
		t.Fatal(err)
	}
	storageTarget, ok := (storage).(oras.Target)
	if !ok {
		t.Fatal("expected oras.Target")
	}
	dummyToken := "DUMMY_TOKEN"
	registryMock := &testRegistryDelegate{
		storage:      storageTarget,
		expectedAuth: dummyToken,
	}
	credFunc := auth.StaticCredential("registry.example.org", auth.Credential{AccessToken: dummyToken})
	_, image1, d, err := registryMock.Resolve("registry.example.org/foobar:v1", false, credFunc)
	if err != nil {
		t.Fatal(err)
	}
	image1 = strings.ReplaceAll(image1, ":v1", "@"+d.Digest.String())

	_, image2, _, err := registryMock.Resolve("registry.example.org/foobar:v2", false, credFunc)
	if err != nil {
		t.Fatal(err)
	}
	delegate := deltadelegate.NewDeltaDelegate()

	type args struct {
		registry    registrydelegate.RegistryDelegate
		delegate    deltadelegate.DeltaDelegate
		apiDelegate testAPIDelegate
		expectErr   bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "valid token",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					token:              dummyToken,
					fromImage:          image1,
					toImage:            image2,
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: false,
			},
		},
		{
			name: "invalid token",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					token:              "invalid token",
					fromImage:          image1,
					toImage:            image2,
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: true,
			},
		},
		{
			name: "no token",
			args: args{
				registry: registryMock,
				delegate: delegate,
				apiDelegate: testAPIDelegate{
					fromImage:          image1,
					toImage:            image2,
					acceptedAlgorithms: []string{"bsdiff", "tardiff", "zstd", "gzip"},
				},
				expectErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The purpose of this loop is to make sure the request has executed fully.
			// This is necessary due to the asynchronous nature of the readDelta function,
			// which spawns a go routine.
			wg := &sync.WaitGroup{}
			ctx := context.WithValue(context.Background(), "wg", wg)
			for {
				readDelta(ctx, tt.args.registry, tt.args.delegate, &tt.args.apiDelegate)
				if tt.args.apiDelegate.hasHandledCallback {
					break
				}
			}
			err = tt.args.apiDelegate.lastErr
			if (err != nil) != tt.args.expectErr {
				t.Fatalf("readDelta() error = %v, wantErr %v", err, tt.args.expectErr)
				return
			}
			response := tt.args.apiDelegate.response
			fmt.Printf("%v\n", response)
		})
	}
}
