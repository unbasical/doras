package differ

import (
	"github.com/unbasical/doras-server/internal/pkg/artifact"
	"testing"
)

func TestNaiveDiffer(t *testing.T) {
	differ := Bsdiff{}
	from := artifact.RawBytesArtifact{Data: []byte("hello")}
	to := artifact.RawBytesArtifact{Data: []byte("hello world")}
	patch := differ.CreateDiff(&from, &to)
	toGot := differ.ApplyDiff(&from, patch)
	if !to.Equals(toGot) {
		t.Fatalf("expected %s, got %s", string(to.Data), string(toGot.GetBytes()))
	}
	toIdentical := artifact.RawBytesArtifact{Data: []byte("hello")}
	patch = differ.CreateDiff(&from, &toIdentical)
	toGot = differ.ApplyDiff(&from, patch)
	if !toIdentical.Equals(toGot) {
		t.Fatalf("expected %s, got %s", string(toIdentical.Data), string(toGot.GetBytes()))
	}
	toCutoff := artifact.RawBytesArtifact{Data: []byte("he")}
	patch = differ.CreateDiff(&from, &toCutoff)
	toGot = differ.ApplyDiff(&from, patch)
	if !toCutoff.Equals(toGot) {
		t.Fatalf("expected %s, got %s", string(toCutoff.Data), string(toGot.GetBytes()))
	}
}
