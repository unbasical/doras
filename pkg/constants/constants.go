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

// QueryKeyAcceptedAlgorithm is used to extract the (repeatable) accepted algorithms parameter from the request.
const QueryKeyAcceptedAlgorithm = "accepted_algorithm"

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

// OciImageTitle is used to extract the titles of container layers from annotations.
const OciImageTitle = "org.opencontainers.image.title"

// Temp-dir/file name prefixes used during patch operations.
//
// These are the single source of truth shared between the temp creation sites
// (the updater client and the delta applier implementations) and the updater's
// startup cleanup routine, so the two cannot drift out of sync if a prefix is
// ever renamed.
//
// Convention: os.MkdirTemp / os.CreateTemp callers append "*" to form the glob
// pattern (e.g. TempPrefixDeltas + "*"). The cleanup routine matches leftover
// entries via strings.HasPrefix using the bare prefix.
const (
	// TempPrefixDeltas is used for the temp dir holding fetched delta artifacts.
	// Created directly under the internal directory.
	TempPrefixDeltas = "deltas-"
	// TempPrefixIntermediate is used for the temp dir holding fetched full-image
	// artifacts before extraction. Created directly under the internal directory.
	TempPrefixIntermediate = "intermediate-"
	// TempPrefixExtract is used for the temp dir a full image is extracted into
	// before it replaces the output directory. Created under the internal directory.
	TempPrefixExtract = "extract-"

	// TempPrefixTarpatch is used for the temp file holding a fully applied tardiff
	// patch. Created under the patcher directory.
	TempPrefixTarpatch = "tarpatch-temp-"
	// TempPrefixTarExtract is used for the temp dir a patched tar is extracted into.
	// Created under the patcher directory.
	TempPrefixTarExtract = "tar-extract-dir-"
	// TempPrefixBsdiff is used for the temp file holding an applied bsdiff patch.
	// Created under the patcher directory.
	TempPrefixBsdiff = "bsdiff-temp-"
)
