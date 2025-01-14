package tardiff

import (
	"errors"
	"github.com/unbasical/doras-server/pkg/algorithm/delta"
	"io"
	"os"

	tardiff "github.com/containers/tar-diff/pkg/tar-diff"
	log "github.com/sirupsen/logrus"
)

type Creator struct {
}

func NewCreator() delta.Differ {
	return &Creator{}
}

func (c *Creator) Diff(old io.Reader, new io.Reader) (io.ReadCloser, error) {
	// parallelize loading the files, as they might be coming from a remote location
	fromFinished := make(chan func() (*os.File, error), 1)
	toFinished := make(chan func() (*os.File, error), 1)

	// This function loads the reader into a temp file
	// and sends a function that produces the file or an error to the provided channel
	loadToTempFile := func(reader io.Reader, fNamePattern string, c chan func() (*os.File, error)) {
		tmpDir := os.TempDir()
		f, err := os.CreateTemp(tmpDir, fNamePattern)
		if err != nil {
			c <- func() (*os.File, error) { return nil, err }
			return
		}
		_, err = io.Copy(f, reader)
		if err != nil {
			_ = os.Remove(f.Name())
			c <- func() (*os.File, error) { return nil, err }
			return
		}
		// reset the file to the start for the consuming function
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			c <- func() (*os.File, error) { return nil, err }
		}
		c <- func() (*os.File, error) { return f, nil }
	}

	// load files in parallel
	go loadToTempFile(old, "from.*.tar.gz", fromFinished)
	go loadToTempFile(new, "to.*.tar.gz", toFinished)
	fpFrom, errFrom := (<-fromFinished)()
	fpTo, errTo := (<-toFinished)()

	// delete both files if we got an error
	err := errors.Join(errFrom, errTo)
	if err != nil {
		if fpFrom != nil {
			_ = os.Remove(fpFrom.Name())
		}
		if fpTo != nil {
			_ = os.Remove(fpTo.Name())
		}
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
				log.WithError(errors.Join(errDiff, errPwClose)).Error("Error closing tar diff")
			}
			return
		}
		err := errors.Join(
			pw.Close(),
			os.Remove(fpFrom.Name()),
			os.Remove(fpTo.Name()),
		)
		if err != nil {
			log.WithError(err).Error("Error during tardiff cleanup")
		}
	}()
	return pr, nil
}

func (c *Creator) Name() string {
	return "tardiff"
}
