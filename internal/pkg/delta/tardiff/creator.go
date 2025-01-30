package tardiff

import (
	"errors"
	"github.com/unbasical/doras/pkg/algorithm/delta"
	"io"
	"os"

	tardiff "github.com/containers/tar-diff/pkg/tar-diff"
	log "github.com/sirupsen/logrus"
)

type Creator struct {
}

// NewCreator returns a tardiff delta.Differ.
func NewCreator() delta.Differ {
	return &Creator{}
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

func (c *Creator) Diff(oldfile io.Reader, newfile io.Reader) (io.ReadCloser, error) {
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
	// finally create a delta
	optsTarDiff := tardiff.NewOptions()
	pr, pw := io.Pipe()
	go func() {
		errDiff := tardiff.Diff(fpFrom, fpTo, pw, optsTarDiff)
		if errDiff != nil {
			errPwClose := pw.CloseWithError(errDiff)
			if errDiff != nil {
				log.WithError(errors.Join(errDiff, errPwClose)).Error("error closing tar diff")
			}
		} else {
			log.Debug("tardiff completed, closing pipe")
			err = pw.Close()
			if err != nil {
				log.WithError(err).Error("error closing tar diff")
			}
		}
		log.Debug("cleaning up temporary files used for tar-diffing")
		err := errors.Join(
			os.Remove(fpFrom.Name()),
			os.Remove(fpTo.Name()),
		)
		if err != nil {
			log.WithError(err).Error("error during tardiff cleanup")
			return
		}
		log.Debug("tardiff cleanup complete")
	}()
	return pr, nil
}

func (c *Creator) Name() string {
	return "tardiff"
}
