package auth

import (
	"encoding/base64"
	"fmt"
)

// GenerateBasicAuth generates a Basic Authentication header value from a username and password.
func GenerateBasicAuth(username, password string) string {
	// Combine username and password with a colon
	credentials := fmt.Sprintf("%s:%s", username, password)

	// Encode the credentials in base64
	encodedCredentials := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Prefix with "Basic " and return
	return "Basic " + encodedCredentials
}
