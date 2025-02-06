package fetcher

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"testing"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func Test_registryImpl_ingest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data),
			Size:   int64(len(data)),
		}
		fPath, err := reg.ingest(d, io.NopCloser(bytes.NewReader(data)))
		if err != nil {
			t.Fatal(err)
			return
		}
		if fPath != path.Join(reg.workingDir, "completed", d.Digest.Encoded()) {
			t.Error("invalid path")
		}
		fp, err := os.Open(fPath)
		if err != nil {
			t.Fatal(err)
			return
		}
		got, err := io.ReadAll(fp)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !bytes.Equal(got, data) {
			t.Errorf("got %q, want %q", got, data)
		}
	})
	t.Run("invalid hash", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data[:2]),
			Size:   int64(len(data)),
		}
		_, err := reg.ingest(d, io.NopCloser(bytes.NewReader(data)))
		if err == nil {
			t.Error("accepted invalid digest")
		}

	})
	t.Run("invalid size", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data),
			Size:   int64(len(data)) - 1,
		}
		_, err := reg.ingest(d, io.NopCloser(bytes.NewReader(data)))
		if err == nil {
			t.Error("accepted invalid size")
		}
	})
	t.Run("pick up partial", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data),
			Size:   int64(len(data)),
		}
		downloadDir, err := reg.ensureSubDir("download")
		if err != nil {
			t.Fatal(err)
		}
		fPath := path.Join(downloadDir, d.Digest.Encoded())
		fp, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fp.Write(data[:3])
		if err != nil {
			t.Fatal(err)
		}
		err = fp.Sync()
		if err != nil {
			t.Error(err)
		}
		seekable := &NopSeeker{Reader: io.NopCloser(bytes.NewReader(data[3:]))}
		fPathGot, err := reg.ingest(d, seekable)
		if err != nil {
			t.Error(err)
		}
		if fPathGot != path.Join(reg.workingDir, "completed", d.Digest.Encoded()) {
			t.Errorf("invalid final path %q", fPathGot)
		}
		fp, err = os.Open(fPathGot)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(fp)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("got %q, want %q", got, data)
		}
		if _, err := os.Stat(fPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("expected file to not exist")
		}
	})
	t.Run("pick up (seeker not implemented)", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data),
			Size:   int64(len(data)),
		}
		downloadDir, err := reg.ensureSubDir("download")
		if err != nil {
			t.Fatal(err)
		}
		fPath := path.Join(downloadDir, d.Digest.Encoded())
		fp, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fp.Write(data[:3])
		if err != nil {
			t.Fatal(err)
		}
		err = fp.Sync()
		if err != nil {
			t.Error(err)
		}
		fPathGot, err := reg.ingest(d, io.NopCloser(bytes.NewReader(data)))
		if err != nil {
			t.Error(err)
		}
		if fPathGot != path.Join(reg.workingDir, "completed", d.Digest.Encoded()) {
			t.Errorf("invalid final path %q", fPathGot)
		}
		fp, err = os.Open(fPathGot)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(fp)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("got %q, want %q", got, data)
		}
		if _, err := os.Stat(fPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("expected file to not exist")
		}
	})
	t.Run("pick up partial invalid digest", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data[:3]),
			Size:   int64(len(data)),
		}
		downloadDir, err := reg.ensureSubDir("download")
		if err != nil {
			t.Fatal(err)
		}
		fPath := path.Join(downloadDir, d.Digest.Encoded())
		fp, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fp.Write(data[:3])
		if err != nil {
			t.Fatal(err)
		}
		err = fp.Sync()
		if err != nil {
			t.Error(err)
		}
		seekable := &NopSeeker{Reader: io.NopCloser(bytes.NewReader(data[3:]))}
		_, err = reg.ingest(d, seekable)
		if err == nil {
			t.Error("accepted invalid digest")
		}
	})
	t.Run("pick up partial invalid length", func(t *testing.T) {
		dir := t.TempDir()
		reg := &registryImpl{workingDir: dir}
		data := []byte("hello world")
		d := v1.Descriptor{
			Digest: digest.FromBytes(data),
			Size:   int64(len(data)),
		}
		downloadDir, err := reg.ensureSubDir("download")
		if err != nil {
			t.Fatal(err)
		}
		fPath := path.Join(downloadDir, d.Digest.Encoded())
		fp, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fp.Write(data[:3])
		if err != nil {
			t.Fatal(err)
		}
		err = fp.Sync()
		if err != nil {
			t.Error(err)
		}
		seekable := &NopSeeker{Reader: io.NopCloser(bytes.NewReader(data[4:]))}
		_, err = reg.ingest(d, seekable)
		if err == nil {
			t.Error("accepted invalid length")
		}
	})
}

type NopSeeker struct {
	Reader io.ReadCloser
}

func (n *NopSeeker) Read(p []byte) (int, error) {
	return n.Reader.Read(p)
}

func (n *NopSeeker) Close() error {
	return n.Reader.Close()
}

func (n *NopSeeker) Seek(_ int64, _ int) (int64, error) {
	// No-op: always return current position (0) and no error
	return 0, nil
}
