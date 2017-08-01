package auth

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
)

func MapGRPCCreds(authToken string) credentials.PerRPCCredentials {
	return &AuthTokenCreds{
		token: authToken,
	}
}

type AuthTokenCreds struct {
	token string
}

func (ts AuthTokenCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "bearer " + ts.token,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (ts AuthTokenCreds) RequireTransportSecurity() bool {
	return true
}
