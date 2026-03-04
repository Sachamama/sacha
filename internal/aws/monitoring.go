package awsx

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/oam"
)

// OAMAPI captures the OAM methods we use for monitoring account detection.
type OAMAPI interface {
	ListSinks(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error)
	ListAttachedLinks(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error)
}

// LinkedAccount represents an AWS account linked to a monitoring account.
type LinkedAccount struct {
	ID    string
	Label string
}

// MonitoringInfo holds monitoring account detection results.
type MonitoringInfo struct {
	IsMonitoring   bool
	LinkedAccounts []LinkedAccount
}

// DetectMonitoringAccount checks if the current account is a CloudWatch
// monitoring account by querying OAM sinks and their attached links.
// Returns MonitoringInfo with linked accounts if monitoring is detected.
func DetectMonitoringAccount(ctx context.Context, cfg aws.Config) MonitoringInfo {
	client := oam.NewFromConfig(cfg)
	return detectMonitoring(ctx, client)
}

func detectMonitoring(ctx context.Context, api OAMAPI) MonitoringInfo {
	sinksOut, err := api.ListSinks(ctx, &oam.ListSinksInput{})
	if err != nil || len(sinksOut.Items) == 0 {
		return MonitoringInfo{}
	}

	sinkARN := sinksOut.Items[0].Arn
	if sinkARN == nil {
		return MonitoringInfo{}
	}

	var accounts []LinkedAccount
	var nextToken *string
	for {
		linksOut, err := api.ListAttachedLinks(ctx, &oam.ListAttachedLinksInput{
			SinkIdentifier: sinkARN,
			NextToken:      nextToken,
		})
		if err != nil {
			break
		}
		for _, link := range linksOut.Items {
			accountID := extractAccountFromLinkARN(aws.ToString(link.LinkArn))
			if accountID == "" {
				continue
			}
			label := aws.ToString(link.Label)
			if label == "" {
				label = accountID
			}
			accounts = append(accounts, LinkedAccount{
				ID:    accountID,
				Label: label,
			})
		}
		if linksOut.NextToken == nil || aws.ToString(linksOut.NextToken) == "" {
			break
		}
		nextToken = linksOut.NextToken
	}

	if len(accounts) == 0 {
		return MonitoringInfo{}
	}

	return MonitoringInfo{
		IsMonitoring:   true,
		LinkedAccounts: accounts,
	}
}

// extractAccountFromLinkARN extracts the account ID from an OAM link ARN.
// Format: arn:aws:oam:REGION:ACCOUNT_ID:link/LINK_ID
func extractAccountFromLinkARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}
