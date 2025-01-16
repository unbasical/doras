package buildurl

import (
	"net/url"
	"strings"
)

// URLBuilder struct to hold the components of the URL
type URLBuilder struct {
	basePath     string
	pathElements []string
	queryParams  url.Values
}

// Option type for functional options
type Option func(*URLBuilder)

// NewURLBuilder creates a new URLBuilder with the given options
func NewURLBuilder(options ...Option) *URLBuilder {
	ub := &URLBuilder{
		queryParams: url.Values{},
	}
	for _, option := range options {
		option(ub)
	}
	return ub
}

// WithBasePath sets the base path of the URL
func WithBasePath(basePath string) Option {
	return func(ub *URLBuilder) {
		ub.basePath = basePath
	}
}

// WithPathElement adds a path element to the URL
func WithPathElement(element string) Option {
	return func(ub *URLBuilder) {
		ub.pathElements = append(ub.pathElements, element)
	}
}

// WithQueryParam adds a query parameter to the URL
func WithQueryParam(key, value string) Option {
	return func(ub *URLBuilder) {
		ub.queryParams.Add(key, value)
	}
}

// WithListQueryParam adds the query parameter multiple times to the URL. Once per element in the values slice.
func WithListQueryParam(key string, values []string) Option {
	return func(ub *URLBuilder) {
		for _, element := range values {
			ub.queryParams.Add(key, element)
		}
	}
}

// Build constructs the final URL string
func (ub *URLBuilder) Build() string {
	var sb strings.Builder
	// We ignore the error because the call returns a nil error according to the doc comment.
	_, _ = sb.WriteString(ub.basePath)
	if len(ub.pathElements) > 0 {
		_, _ = sb.WriteString("/")
		_, _ = sb.WriteString(strings.Join(ub.pathElements, "/"))
	}
	if len(ub.queryParams) > 0 {
		_, _ = sb.WriteString("?")
		_, _ = sb.WriteString(ub.queryParams.Encode())
	}
	return sb.String()
}

func New(options ...Option) string {
	return NewURLBuilder(options...).Build()
}
