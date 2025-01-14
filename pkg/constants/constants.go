package constants

// DorasAnnotationFrom is the constant to extract the from-image from the delta image's manifest.
const DorasAnnotationFrom = "com.unbasical.doras.delta.from"

// DorasAnnotationTo is the constant to extract the to-image from the delta image's manifest.
const DorasAnnotationTo = "com.unbasical.doras.delta.to"

// DorasAnnotationIsDummy is the constant to extract the information of whether the artifact is a dummy from the image's manifest.
const DorasAnnotationIsDummy = "com.unbasical.doras.delta.dummy"

// QueryKeyFromDigest is used to extract the from_digest parameter from the request.
const QueryKeyFromDigest = "from_digest"

// QueryKeyToTag is used to extract the to_tag parameter from the request.
const QueryKeyToTag = "to_tag"

// QueryKeyToDigest is used to extract the to_digest parameter from the request.
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
