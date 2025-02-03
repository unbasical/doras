package patchapplier

import (
	"github.com/unbasical/doras/pkg/algorithm/compression"
	"github.com/unbasical/doras/pkg/algorithm/delta"
)

type PatchApplier interface {
	Patch(targetImage, deltaPath, outDir string) (string, error)
}

func NewPatchApplier(algo string) (PatchApplier, error) {
	applier := patchApplier{}
	switch algo {
	default:
		panic("todo")
	}
	return &applier, nil
}

type patchApplier struct {
	compression.Decompressor
	delta.Patcher
}

func (p *patchApplier) Patch(targetImage, deltaPath, outDir string) (string, error) {
	//TODO implement me
	panic("implement me")
}
