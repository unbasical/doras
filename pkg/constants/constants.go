package constants

const DorasAnnotationFrom = "com.unbasical.doras.delta.from"
const DorasAnnotationTo = "com.unbasical.doras.delta.to"
const DorasAnnotationIsDummy = "com.unbasical.doras.delta.dummy"

const QueryKeyFromDigest = "from_digest"
const QueryKeyToTag = "to_tag"
const QueryKeyToDigest = "to_digest"

// DefaultAlgorithms returns the Doras default algorithms.
// Uses a function to ensure immutability of the default algorithms.
func DefaultAlgorithms() []string {
	return []string{
		"bsdiff",
		"tardiff",
		"zstd",
	}
}

// OrasContentUnpack is used to extract metadata from an OCI manifest.
// Indicates whether the given artifact is an archive.
const OrasContentUnpack = "io.deis.oras.content.unpack"
