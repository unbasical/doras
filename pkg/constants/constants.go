package constants

const DorasAnnotationFrom = "from"
const DorasAnnotationTo = "to"
const DorasAnnotationAlgorithm = "algorithm"
const DorasAnnotationIsDummy = "dummy"

const QueryKeyFromDigest = "from_digest"
const QueryKeyToTag = "to_tag"
const QueryKeyToDigest = "to_digest"

func DefaultAlgorithms() []string {
	return []string{
		"bsdiff",
		"tardiff",
		"zstd",
	}
}

const ContentUnpack = "io.deis.oras.content.unpack"
