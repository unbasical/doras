package ociutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/idna"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// ParseManifestJSON return the v1.manifest that is yielded by the reader.
func ParseManifestJSON(data io.Reader) (*v1.Manifest, error) {
	var manifest *v1.Manifest
	err := json.NewDecoder(data).Decode(&manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

var reDigest = regexp.MustCompile(`\S*@sha256:[a-f0-9]{64}$`)

// IsDigest returns if the image is identified by a digest.
func IsDigest(imageOrTag string) bool {
	return reDigest.MatchString(imageOrTag)
}

const patternOCIImage = `^/([a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*)((:([a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}))|(@sha256:[a-f0-9]{64}))$`

var regexOCIImage = regexp.MustCompile(patternOCIImage)

// ParseOciImageString splits the image string into the repo, tag
// and indicates if it is an image that is identified by a digest.
func ParseOciImageString(r string) (repoName string, tag string, isDigest bool, err error) {
	if !strings.HasPrefix(r, "oci://") {
		r = "oci://" + r
	}
	u, err := url.Parse(r)
	if err != nil {
		return "", "", false, err
	}
	matches := regexOCIImage.FindSubmatch([]byte(u.Path))
	if matches == nil {
		return "", "", false, errors.New("invalid OCI image")
	}
	if repoName = string(matches[1]); repoName == "" {
		return "", "", false, errors.New("invalid OCI image")
	}
	if tag = string(matches[9]); tag == "" {
		if tag = string(matches[10]); tag == "" {
			return "", "", false, errors.New("invalid OCI image")
		}
		isDigest = true
	}
	hostname, err := canonicalizeHostname(u.Host)
	if err != nil {
		return "", "", false, err
	}
	repoName = fmt.Sprintf("%s/%s", hostname, repoName)
	return
}

func ParseOciUrl(rawURL string) (*url.URL, error) {
	if !strings.Contains(rawURL, "://") {
		rawURL = "oci://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return parsedURL, nil
}

func CheckRegistryMatch(a, b string) error {
	u1, err := ParseOciUrl(a)
	if err != nil {
		return err
	}
	u2, err := ParseOciUrl(b)
	if err != nil {
		return err
	}
	hostName1, err := canonicalizeHostname(u1.Host)
	if err != nil {
		return err
	}
	hostName2, err := canonicalizeHostname(u2.Host)
	if err != nil {
		return err
	}
	if hostName1 != hostName2 {
		return errors.New("registry hosts do not match")
	}
	return nil
}

// canonicalizeHostname standardizes a hostname to ensure consistent representation.
// It performs the following steps:
// 1. Converts the hostname to lowercase (DNS is case-insensitive).
// 2. Removes any trailing dot (equivalent in DNS resolution).
// 3. Encodes the hostname in Punycode if it contains non-ASCII characters (to support IDNs).
// This ensures the hostname is represented in a format suitable for DNS lookups or comparisons.
//
// Parameters:
//
//	hostname (string): The input hostname.
//
// Returns:
//
//	(string, error): The canonicalized hostname or an error if the conversion fails.
func canonicalizeHostname(hostname string) (string, error) {
	// Convert to lowercase
	hostname = strings.ToLower(hostname)

	// Remove trailing dot
	hostname = strings.TrimSuffix(hostname, ".")

	// Convert to Punycode
	punycode, err := idna.ToASCII(hostname)
	if err != nil {
		return "", err
	}

	return punycode, nil
}
