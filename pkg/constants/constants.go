package constants

const DorasAnnotationFrom = "from"
const DorasAnnotationTo = "to"
const DorasAnnotationAlgorithm = "algorithm"

const QueryKeyFromDigest = "from_digest"
const QueryKeyToTag = "to_tag"
const QueryKeyToDigest = "to_digest"

func DefaultAlgorithms() []string {
	return []string{
		"bsdiff",
		"tardiff",
	}
}
