/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci_file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/errdef"
)

func TestStorage_Success(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	for _, f := range []bool{true, false} {
		tempDir := t.TempDir()
		cacheRoot := filepath.Join(tempDir, "cache")
		fsRoot := filepath.Join(tempDir, "fs")
		s, err := NewStorage(cacheRoot, fsRoot, f)
		if err != nil {
			t.Fatal("New() error =", err)
		}
		ctx := context.Background()

		// test push
		err = s.Push(ctx, desc, bytes.NewReader(content))
		if err != nil {
			t.Fatal("Storage.Push() error =", err)
		}

		// test fetch
		exists, err := s.Exists(ctx, desc)
		if err != nil {
			t.Fatal("Storage.Exists() error =", err)
		}
		if !exists {
			t.Errorf("Storage.Exists() = %v, want %v", exists, true)
		}

		rc, err := s.Fetch(ctx, desc)
		if err != nil {
			t.Fatal("Storage.Fetch() error =", err)
		}
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal("Storage.Fetch().Read() error =", err)
		}
		err = rc.Close()
		if err != nil {
			t.Error("Storage.Fetch().Close() error =", err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("Storage.Fetch() = %v, want %v", got, content)
		}
	}

}

func TestStorage_RelativeRoot_Success(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}

	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("error calling filepath.EvalSymlinks(), error =", err)
	}
	cacheRoot := filepath.Join(tempDir, "cache")
	fsRoot := filepath.Join(tempDir, "fs")
	currDir, err := os.Getwd()
	if err != nil {
		t.Fatal("error calling Getwd(), error=", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal("error calling Chdir(), error=", err)
	}
	s, err := NewStorage("./cache", "./fs", false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	if want := cacheRoot; s.cacheRoot != want {
		t.Errorf("Storage.cacheRoot = %s, want %s", s.cacheRoot, want)
	}
	if want := fsRoot; s.fsRoot != want {
		t.Errorf("Storage.fsRoot = %s, want %s", s.cacheRoot, want)
	}
	// cd back to allow the temp directory to be removed
	if err := os.Chdir(currDir); err != nil {
		t.Fatal("error calling Chdir(), error=", err)
	}
	ctx := context.Background()

	// test push
	err = s.Push(ctx, desc, bytes.NewReader(content))
	if err != nil {
		t.Fatal("Storage.Push() error =", err)
	}

	// test fetch
	exists, err := s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if !exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, true)
	}

	rc, err := s.Fetch(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Fetch() error =", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal("Storage.Fetch().Read() error =", err)
	}
	err = rc.Close()
	if err != nil {
		t.Error("Storage.Fetch().Close() error =", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("Storage.Fetch() = %v, want %v", got, content)
	}
}

func TestStorage_NotFound(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	for _, f := range []bool{true, false} {
		tempDir := t.TempDir()
		cacheRoot := filepath.Join(tempDir, "cache")
		fsRoot := filepath.Join(tempDir, "fs")
		s, err := NewStorage(cacheRoot, fsRoot, f)
		if err != nil {
			t.Fatal("New() error =", err)
		}
		ctx := context.Background()

		exists, err := s.Exists(ctx, desc)
		if err != nil {
			t.Error("Storage.Exists() error =", err)
		}
		if exists {
			t.Errorf("Storage.Exists() = %v, want %v", exists, false)
		}

		_, err = s.Fetch(ctx, desc)
		if !errors.Is(err, errdef.ErrNotFound) {
			t.Errorf("Storage.Fetch() error = %v, want %v", err, errdef.ErrNotFound)
		}
	}

}

func TestStorage_AlreadyExists(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	for _, f := range []bool{true, false} {
		tempDir := t.TempDir()
		cacheRoot := filepath.Join(tempDir, "cache")
		fsRoot := filepath.Join(tempDir, "fs")
		s, err := NewStorage(cacheRoot, fsRoot, f)
		if err != nil {
			t.Fatal("New() error =", err)
		}
		ctx := context.Background()

		err = s.Push(ctx, desc, bytes.NewReader(content))
		if err != nil {
			t.Fatal("Storage.Push() error =", err)
		}

		err = s.Push(ctx, desc, bytes.NewReader(content))
		if !errors.Is(err, errdef.ErrAlreadyExists) {
			t.Errorf("Storage.Push() error = %v, want %v", err, errdef.ErrAlreadyExists)
		}
	}

}

func TestStorage_BadPush(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	for _, f := range []bool{true, false} {
		tempDir := t.TempDir()
		cacheRoot := filepath.Join(tempDir, "cache")
		fsRoot := filepath.Join(tempDir, "fs")
		s, err := NewStorage(cacheRoot, fsRoot, f)
		if err != nil {
			t.Fatal("New() error =", err)
		}
		ctx := context.Background()

		err = s.Push(ctx, desc, strings.NewReader("foobar"))
		if err == nil {
			t.Errorf("Storage.Push() error = %v, wantErr %v", err, true)
		}
	}
}

func TestStorage_Push_Concurrent(t *testing.T) {
	content := []byte("hello world")
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	for _, f := range []bool{true, false} {
		tempDir := t.TempDir()
		cacheRoot := filepath.Join(tempDir, "cache")
		fsRoot := filepath.Join(tempDir, "fs")
		s, err := NewStorage(cacheRoot, fsRoot, f)
		if err != nil {
			t.Fatal("New() error =", err)
		}
		ctx := context.Background()

		concurrency := 64
		eg, egCtx := errgroup.WithContext(ctx)
		for i := 0; i < concurrency; i++ {
			eg.Go(func(i int) func() error {
				return func() error {
					if err := s.Push(egCtx, desc, bytes.NewReader(content)); err != nil {
						if errors.Is(err, errdef.ErrAlreadyExists) {
							return nil
						}
						return fmt.Errorf("failed to push content: %v", err)
					}
					return nil
				}
			}(i))
		}

		if err := eg.Wait(); err != nil {
			t.Fatal(err)
		}

		exists, err := s.Exists(ctx, desc)
		if err != nil {
			t.Fatal("Storage.Exists() error =", err)
		}
		if !exists {
			t.Errorf("Storage.Exists() = %v, want %v", exists, true)
		}

		rc, err := s.Fetch(ctx, desc)
		if err != nil {
			t.Fatal("Storage.Fetch() error =", err)
		}
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal("Storage.Fetch().Read() error =", err)
		}
		err = rc.Close()
		if err != nil {
			t.Error("Storage.Fetch().Close() error =", err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("Storage.Fetch() = %v, want %v", got, content)
		}
	}

}

func TestStorage_Fetch_ExistingBlobs(t *testing.T) {
	content := []byte("hello world")
	dgst := digest.FromBytes(content)
	const fName = "foo"
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    dgst,
		Size:      int64(len(content)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: fName,
		},
	}

	tempDir := t.TempDir()
	cacheRoot := filepath.Join(tempDir, "cache")
	fsRoot := filepath.Join(tempDir, "fs")
	blobPath := filepath.Join(cacheRoot, "blobs", dgst.Algorithm().String(), dgst.Encoded())
	outputFilePath := filepath.Join(fsRoot, fName)
	if err := os.MkdirAll(filepath.Dir(blobPath), 0777); err != nil {
		t.Fatal("error calling Mkdir(), error =", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0777); err != nil {
		t.Fatal("error calling Mkdir(), error =", err)
	}
	if err := os.WriteFile(blobPath, content, 0444); err != nil {
		t.Fatal("error calling WriteFile(), error =", err)
	}
	if err := os.WriteFile(outputFilePath, content, 0444); err != nil {
		t.Fatal("error calling WriteFile(), error =", err)
	}

	s, err := NewStorage(cacheRoot, fsRoot, false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	ctx := context.Background()

	exists, err := s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if !exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, true)
	}

	rc, err := s.Fetch(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Fetch() error =", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal("Storage.Fetch().Read() error =", err)
	}
	err = rc.Close()
	if err != nil {
		t.Error("Storage.Fetch().Close() error =", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("Storage.Fetch() = %v, want %v", got, content)
	}
}

func TestStorage_Exists_FileMissesInCache(t *testing.T) {
	content := []byte("hello world")
	dgst := digest.FromBytes(content)
	const fName = "foo"
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    dgst,
		Size:      int64(len(content)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: fName,
		},
	}

	tempDir := t.TempDir()
	cacheRoot := filepath.Join(tempDir, "cache")
	fsRoot := filepath.Join(tempDir, "fs")
	outputFilePath := filepath.Join(fsRoot, fName)
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0777); err != nil {
		t.Fatal("error calling Mkdir(), error =", err)
	}
	if err := os.WriteFile(outputFilePath, content, 0444); err != nil {
		t.Fatal("error calling WriteFile(), error =", err)
	}

	s, err := NewStorage(cacheRoot, fsRoot, false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	ctx := context.Background()

	exists, err := s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, false)
	}
}

func TestStorage_Exists_FileMissesInOutput(t *testing.T) {
	content := []byte("hello world")
	dgst := digest.FromBytes(content)
	const fName = "foo"
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    dgst,
		Size:      int64(len(content)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: fName,
		},
	}

	tempDir := t.TempDir()
	cacheRoot := filepath.Join(tempDir, "cache")
	fsRoot := filepath.Join(tempDir, "fs")
	blobPath := filepath.Join(cacheRoot, "blobs", dgst.Algorithm().String(), dgst.Encoded())

	if err := os.MkdirAll(filepath.Dir(blobPath), 0777); err != nil {
		t.Fatal("error calling Mkdir(), error =", err)
	}
	if err := os.WriteFile(blobPath, content, 0444); err != nil {
		t.Fatal("error calling WriteFile(), error =", err)
	}

	s, err := NewStorage(cacheRoot, fsRoot, false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	ctx := context.Background()

	exists, err := s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, false)
	}
}

func TestStorage_Fetch_Concurrent(t *testing.T) {
	content := []byte("hello world")
	const fName = "foo"
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: fName,
		},
	}

	tempDir := t.TempDir()
	s, err := NewStorage(tempDir, t.TempDir(), false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	ctx := context.Background()

	if err := s.Push(ctx, desc, bytes.NewReader(content)); err != nil {
		t.Fatal("Storage.Push() error =", err)
	}

	concurrency := 64
	eg, egCtx := errgroup.WithContext(ctx)

	for i := 0; i < concurrency; i++ {
		eg.Go(func(i int) func() error {
			return func() error {
				rc, err := s.Fetch(egCtx, desc)
				if err != nil {
					return fmt.Errorf("failed to fetch content: %v", err)
				}
				got, err := io.ReadAll(rc)
				if err != nil {
					t.Fatal("Storage.Fetch().Read() error =", err)
				}
				err = rc.Close()
				if err != nil {
					t.Error("Storage.Fetch().Close() error =", err)
				}
				if !bytes.Equal(got, content) {
					t.Errorf("Storage.Fetch() = %v, want %v", got, content)
				}
				return nil
			}
		}(i))
	}

	if err := eg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestStorage_Delete(t *testing.T) {
	content := []byte("test delete")
	const fName = "foo"
	desc := ocispec.Descriptor{
		MediaType: "test",
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: fName,
		},
	}
	tempDir := t.TempDir()
	cacheRoot := filepath.Join(tempDir, "cache")
	fsRoot := filepath.Join(tempDir, "fs")
	blobPath := filepath.Join(cacheRoot, "blobs", desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	outputFilePath := filepath.Join(fsRoot, fName)

	s, err := NewStorage(cacheRoot, fsRoot, false)
	if err != nil {
		t.Fatal("New() error =", err)
	}
	ctx := context.Background()
	if err := s.Push(ctx, desc, bytes.NewReader(content)); err != nil {
		t.Fatal("Storage.Push() error =", err)
	}
	exists, err := s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if !exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, true)
	}
	err = s.Delete(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Delete() error =", err)
	}
	exists, err = s.Exists(ctx, desc)
	if err != nil {
		t.Fatal("Storage.Exists() error =", err)
	}
	if exists {
		t.Errorf("Storage.Exists() = %v, want %v", exists, false)
	}
	err = s.Delete(ctx, desc)
	if !errors.Is(err, errdef.ErrNotFound) {
		t.Fatalf("got error = %v, want %v", err, errdef.ErrNotFound)
	}
	if _, err := os.Stat(blobPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file was not deleted in cache")
	}
	if _, err := os.Stat(outputFilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file was not deleted in output folder")
	}
}
