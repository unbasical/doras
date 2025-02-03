package verifier

// ArtifactVerifier ensures update integrity before applying.
type ArtifactVerifier interface {
	VerifyArtifact(targetImage, artifactPath string) error // Verify against digest
}
