package tarfsdatasource

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"testing"
)

func Test_General_Functioning(t *testing.T) {
	// Create and add some files to the archive.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	f1Content := "This archive contains some text files."
	f2Content := "Gopher names:\nGeorge\nGeoffrey\nGonzo"
	f3Content := "Get animal handling license."
	var files = []struct {
		Name, Body string
	}{
		{"readme.txt", f1Content},
		{"gopher.txt", f2Content},
		{"todo.txt", f3Content},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dataSource, err := NewDataSource(&buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files[0:] {
		err := dataSource.SetCurrentFile(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(dataSource)
		if err != nil && !errors.Is(err, io.EOF) {
			t.Error(err)
		}
		if !bytes.Equal(got, []byte(file.Body)) {
			t.Errorf("got %q,\nwant %q", string(got), file.Body)
		}
	}

}
