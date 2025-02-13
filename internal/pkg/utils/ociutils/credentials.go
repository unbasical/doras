package ociutils

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type CredFuncAggregate struct {
	credFuncs []auth.CredentialFunc
}

// NewCredentialsAggregate aggregates multiple auth.CredentialFunc into one function.
// Loads in the order they were added and returns an error if none are found.
func NewCredentialsAggregate(opts ...func(aggregate *CredFuncAggregate)) auth.CredentialFunc {
	aggregate := &CredFuncAggregate{}
	for _, opt := range opts {
		opt(aggregate)
	}
	return func(ctx context.Context, hostport string) (auth.Credential, error) {
		for _, credFunc := range aggregate.credFuncs {
			cred, err := credFunc(ctx, hostport)
			if err == nil {
				logrus.Infof("found credential for %s:", hostport)
				return cred, nil
			}
			logrus.WithError(err).Debugf("failed to load credentials for %v", hostport)
		}
		return auth.Credential{}, errors.New("no credential function found")
	}
}

func WithCredFunc(cf auth.CredentialFunc) func(*CredFuncAggregate) {
	return func(c *CredFuncAggregate) {
		c.credFuncs = append(c.credFuncs, cf)
	}
}
