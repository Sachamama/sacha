package awsx

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/oam"
)

// OAMAPI captures the AWS OAM methods we use.
type OAMAPI interface {
	ListSinks(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error)
	ListAttachedLinks(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error)
}

// MonitoringInfo holds cross-account observability metadata.
type MonitoringInfo struct {
	IsMonitoring   bool
	SinkARN        string
	LinkedAccounts []LinkedAccount
}

// LinkedAccount represents a source account linked to the monitoring account.
type LinkedAccount struct {
	AccountID string
	Label     string
}

// DetectMonitoringAccount checks if the current account is a CloudWatch
// monitoring account by looking for OAM sinks. If a sink exists, it fetches
// the linked source accounts.
func DetectMonitoringAccount(ctx context.Context, cfg aws.Config) MonitoringInfo {
	client := oam.NewFromConfig(cfg)
	return detectMonitoringWithAPI(ctx, client)
}

func detectMonitoringWithAPI(ctx context.Context, api OAMAPI) MonitoringInfo {
	sinksOut, err := api.ListSinks(ctx, &oam.ListSinksInput{})
	if err != nil || len(sinksOut.Items) == 0 {
		return MonitoringInfo{}
	}

	sinkARN := ""
	if sinksOut.Items[0].Arn != nil {
		sinkARN = *sinksOut.Items[0].Arn
	}

	info := MonitoringInfo{
		IsMonitoring: true,
		SinkARN:      sinkARN,
	}

	if sinkARN == "" {
		return info
	}

	linksOut, err := api.ListAttachedLinks(ctx, &oam.ListAttachedLinksInput{
		SinkIdentifier: &sinkARN,
	})
	if err != nil {
		return info
	}

	for _, link := range linksOut.Items {
		acct := LinkedAccount{}
		if link.Label != nil {
			acct.Label = *link.Label
			acct.AccountID = *link.Label
		}
		if acct.AccountID != "" {
			info.LinkedAccounts = append(info.LinkedAccounts, acct)
		}
	}

	return info
}
