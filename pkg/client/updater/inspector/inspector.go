package inspector

import (
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/internal/pkg/utils/observer"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/internal/pkg/utils/readerutils"
	"github.com/unbasical/doras/pkg/client/updater/validator"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ArtifactInspector defines methods to inspect OCI artifacts' manifest and contents.
// InspectManifest inspects the provided OCI manifest to evaluate its metadata or attributes.
// InspectContents analyzes the artifact contents from the given reader and returns a pass-through reader or an error.
type ArtifactInspector interface {
	InspectManifest(mf *ociutils.Manifest) error
	InspectContents(rc io.ReadCloser) (io.ReadCloser, error)
}

// DownloadStatsObserver observes and records download statistics during artifact fetching.
type DownloadStatsObserver struct {
	bytesRead    atomic.Uint64
	statsDirPath string
}

// NewDownloadStatsObserver creates a new DownloadStatsObserver with the provided stats directory path.
func NewDownloadStatsObserver(statsDirPath string) *DownloadStatsObserver {
	return &DownloadStatsObserver{
		statsDirPath: statsDirPath,
	}
}

// InspectManifest in this case is a NOP.
func (d *DownloadStatsObserver) InspectManifest(_ *ociutils.Manifest) error {
	return nil
}

// InspectContents wraps a provided io.ReadCloser to track bytes read and observe download stats with periodic updates.
func (d *DownloadStatsObserver) InspectContents(rc io.ReadCloser) (io.ReadCloser, error) {
	d.bytesRead.Store(0)
	stop := make(chan any)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := ObserveDownloadStats(d.statsDirPath, &d.bytesRead, stop)
		if err != nil {
			log.Errorf("failed to observe download stats: %v", err)
		}
	}()
	retval := readerutils.NewCleanupReadCloser(
		readerutils.NewCountingReader(rc, &d.bytesRead),
		func() error {
			close(stop)
			wg.Wait()
			return nil
		},
	)
	return retval, nil
}

// ObserveDownloadStats periodically observes an uint64 atomic value and writes it to a file in the specified directory.
// dir specifies the directory where the observation results are stored.
// p is a pointer to the atomic uint64 value to be observed.
// stop is a channel to signal when the periodic observation should stop.
// Returns an error if any occurs during the observation or file writing process.
func ObserveDownloadStats(dir string, p *atomic.Uint64, stop <-chan any) error {
	o := observer.IntervalObserver[*atomic.Uint64]{
		Interval: funcutils.Unwrap(time.ParseDuration("15s")),
		F: func(p *atomic.Uint64) error {
			loaded := p.Swap(0)
			return validator.WriteUintToFile(dir, loaded)
		},
		Observable: p,
	}
	return o.Observe(stop)
}
