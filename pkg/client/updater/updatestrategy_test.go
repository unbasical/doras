package updater

import (
	"errors"
	"github.com/unbasical/doras/pkg/client/updater/backupmanager"
	ociutils "github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/healthchecker"
	"github.com/unbasical/doras/pkg/client/updater/storage"
	"github.com/unbasical/doras/pkg/client/updater/verifier"
	"os"
	"path"
	"testing"
)

func Test_update(t *testing.T) {
	s, err := newTestStorage(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	type args struct {
		image       string
		finder      testUpdateFinder
		s           storage.Storage
		f           ociutils.ArtifactFetcher
		verifiers   []verifier.ArtifactVerifier
		b           backupmanager.BackupManager
		u           storage.Updater
		h           healthchecker.HealthChecker
		forceUpdate bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "", args: args{
			image: "registry.example.org/image:full",
			finder: testUpdateFinder{
				deltaImage: "registry.example.org/image:delta",
				fullImage:  "registry.example.org/image:full",
			},
			s:           s,
			f:           nil,
			verifiers:   nil,
			b:           nil,
			u:           &testUpdater{},
			h:           nil,
			forceUpdate: false,
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := update(tt.args.image, &tt.args.finder, tt.args.s, tt.args.f, tt.args.verifiers, tt.args.b, tt.args.u, tt.args.h, tt.args.forceUpdate); (err != nil) != tt.wantErr {
				t.Errorf("update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type testUpdateFinder struct {
	deltaImage string
	fullImage  string
}

func (t *testUpdateFinder) LoadUpdate(_ string, isInitialized bool) (isDelta bool, updateImage string, err error) {
	if isInitialized {
		return true, t.deltaImage, nil
	}
	return false, t.fullImage, nil
}

type testStorage struct {
	baseDir        string
	currentVersion string
}

func newTestStorage(baseDir string) (storage.Storage, error) {
	t := testStorage{baseDir: baseDir}
	dirs := []string{
		baseDir,
		path.Join(baseDir, "deltas"),
		path.Join("intermediate"),
		path.Join("artifacts"),
		path.Join("backups"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0700)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return &t, nil
}

func (t *testStorage) DeltaDirectory() string {
	return path.Join(t.baseDir, "delta")
}

func (t *testStorage) IntermediateDirectory() string {
	return path.Join(t.baseDir, "intermediate")
}

func (t *testStorage) ArtifactDirectory() string {
	return path.Join(t.baseDir, "artifacts")
}

func (t *testStorage) BackupDirectory() string {
	return path.Join(t.baseDir, "backups")
}

func (t *testStorage) CurrentVersion() (string, bool) {
	if t.currentVersion != "" {
		return t.currentVersion, true
	}
	return "", false
}

type testUpdater struct {
	failedUpdates map[string]any
}

func (t *testUpdater) ApplyUpdate(outDir, newArtifactPath string) error {
	return nil
}

func (t *testUpdater) Rollback(b backupmanager.BackupManager) error {
	return nil
}

func (t *testUpdater) MarkFailed(version string) {
	t.failedUpdates[version] = nil
}

func (t *testUpdater) HasFailed(version string) bool {
	_, ok := t.failedUpdates[version]
	return ok
}

type testFetcher struct {
}

func (t *testFetcher) FetchDelta(currentVersion, image, outDir string) (algo, fPath string, err error) {
	//TODO implement me
	panic("implement me")
}

func (t *testFetcher) FetchArtifact(image, outDir string) (fPath string, err error) {
	//TODO implement me
	panic("implement me")
}
