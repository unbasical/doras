package tardiff

import (
	"errors"
	tardiff "github.com/unbasical/doras/internal/pkg/utils/differutils/tar-diff"
	"github.com/unbasical/doras/internal/pkg/utils/readerutils"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

type differ struct {
}

// NewCreator returns a tardiff delta.Differ.
func NewCreator() delta.Differ {
	return &differ{}
}

// loadToTempFile and sends a function that produces the file or an error to the provided channel
func loadToTempFile(reader io.Reader, fNamePattern string, c chan func() (*os.File, error)) {
	tmpDir := os.TempDir()
	f, err := os.CreateTemp(tmpDir, fNamePattern)
	if err != nil {
		c <- func() (*os.File, error) { return nil, err }
		return
	}
	n, err := io.Copy(f, reader)
	if err != nil {
		log.WithError(err).Error("failed to copy file to temp file, removing ...")
		_ = os.Remove(f.Name())
		c <- func() (*os.File, error) { return nil, err }
		return
	}
	log.Debugf("wrote %d bytes to temporary file", n)
	// reset the file to the start for the consuming function
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		c <- func() (*os.File, error) { return nil, err }
	}
	c <- func() (*os.File, error) { return f, nil }
}

func (c *differ) Diff(oldfile io.Reader, newfile io.Reader) (io.ReadCloser, error) {
	// parallelize loading the files, as they might be coming from a remote location
	fromFinished := make(chan func() (*os.File, error), 1)
	toFinished := make(chan func() (*os.File, error), 1)

	// load files in parallel
	go loadToTempFile(oldfile, "from.*.tar.gz", fromFinished)
	go loadToTempFile(newfile, "to.*.tar.gz", toFinished)
	fpFrom, errFrom := (<-fromFinished)()
	fpTo, errTo := (<-toFinished)()

	// delete both files if we got an error
	err := errors.Join(errFrom, errTo)
	if err != nil {
		log.WithError(err).Debug("encountered error while loading input tars, cleaning up")
		if fpFrom != nil {
			_ = os.Remove(fpFrom.Name())
		}
		if fpTo != nil {
			_ = os.Remove(fpTo.Name())
		}
		log.Debug("cleaned up temporary files")
		return nil, err
	}
	// make sure temp files are cleaned up
	defer func() {
		log.Debug("cleaned up tardiff input temp files")
		errCleanup := errors.Join(
			fpFrom.Close(),
			fpTo.Close(),
			os.Remove(fpFrom.Name()),
			os.Remove(fpTo.Name()),
		)
		if errCleanup != nil {
			log.WithError(errCleanup).Debug("encountered error while cleaning input temp files for tardiff")
		}
	}()
	// finally create a delta
	optsTarDiff := tardiff.NewOptions()
	tmpDir := os.TempDir()
	fpW, err := os.CreateTemp(tmpDir, "*.tardiff")
	if err != nil {
		return nil, err
	}
	err = tardiff.Diff(fpFrom, fpTo, fpW, optsTarDiff)
	if err != nil {
		log.WithError(err).Debug("encountered error while creating tardiff, cleaning up")
		errCleanup := errors.Join(
			fpW.Close(),
			os.Remove(fpW.Name()),
		)
		if err != nil {
			log.WithError(errCleanup).Debug("encountered error while cleaning up tardiff")
		}
		return nil, err
	}
	// fetch file size for logging purposes
	stats, err := os.Stat(fpW.Name())
	if err != nil {
		return nil, err
	}
	log.Debugf("wrote tardiff with %d bytes to temporary file", stats.Size())
	_, err = fpW.Seek(0, io.SeekStart)
	if err != nil {
		log.WithError(err).Debug("encountered error while seeking temp file used for tardiff")
		return nil, err
	}
	rc := readerutils.NewCleanupReadCloser(fpW, func() error {
		return os.Remove(fpW.Name())
	})
	return rc, nil
}

func (c *differ) Name() string {
	return "tardiff"
}
