package awsx

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ResolveAccountID calls STS GetCallerIdentity to determine the AWS account ID.
// Returns the account ID string on success. If the call fails, it returns the
// provided fallback value (typically the profile name).
func ResolveAccountID(ctx context.Context, cfg aws.Config, fallback string) string {
	client := sts.NewFromConfig(cfg)
	out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || out.Account == nil {
		if fallback == "" {
			return "default"
		}
		return fallback
	}
	return *out.Account
}
