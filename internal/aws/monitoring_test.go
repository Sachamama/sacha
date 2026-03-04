package awsx

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/oam"
	oamtypes "github.com/aws/aws-sdk-go-v2/service/oam/types"
)

type mockOAMAPI struct {
	listSinksFn         func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error)
	listAttachedLinksFn func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error)
}

func (m *mockOAMAPI) ListSinks(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
	if m.listSinksFn != nil {
		return m.listSinksFn(ctx, params, optFns...)
	}
	return &oam.ListSinksOutput{}, nil
}

func (m *mockOAMAPI) ListAttachedLinks(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
	if m.listAttachedLinksFn != nil {
		return m.listAttachedLinksFn(ctx, params, optFns...)
	}
	return &oam.ListAttachedLinksOutput{}, nil
}

func TestDetectMonitoring_NoSinks(t *testing.T) {
	mock := &mockOAMAPI{
		listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
			return &oam.ListSinksOutput{Items: []oamtypes.ListSinksItem{}}, nil
		},
	}
	info := detectMonitoring(context.Background(), mock)
	if info.IsMonitoring {
		t.Error("expected IsMonitoring to be false when no sinks exist")
	}
	if len(info.LinkedAccounts) != 0 {
		t.Errorf("expected 0 linked accounts, got %d", len(info.LinkedAccounts))
	}
}

func TestDetectMonitoring_SinksError(t *testing.T) {
	mock := &mockOAMAPI{
		listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	info := detectMonitoring(context.Background(), mock)
	if info.IsMonitoring {
		t.Error("expected IsMonitoring to be false on error")
	}
}

func TestDetectMonitoring_WithLinkedAccounts(t *testing.T) {
	mock := &mockOAMAPI{
		listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
			return &oam.ListSinksOutput{
				Items: []oamtypes.ListSinksItem{
					{Arn: aws.String("arn:aws:oam:us-east-1:111111111111:sink/sink-id")},
				},
			}, nil
		},
		listAttachedLinksFn: func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
			if aws.ToString(params.SinkIdentifier) != "arn:aws:oam:us-east-1:111111111111:sink/sink-id" {
				t.Errorf("unexpected sink identifier: %s", aws.ToString(params.SinkIdentifier))
			}
			return &oam.ListAttachedLinksOutput{
				Items: []oamtypes.ListAttachedLinksItem{
					{
						LinkArn: aws.String("arn:aws:oam:us-east-1:222222222222:link/link-1"),
						Label:   aws.String("dev-account"),
					},
					{
						LinkArn: aws.String("arn:aws:oam:us-east-1:333333333333:link/link-2"),
						Label:   aws.String("staging-account"),
					},
				},
			}, nil
		},
	}

	info := detectMonitoring(context.Background(), mock)
	if !info.IsMonitoring {
		t.Fatal("expected IsMonitoring to be true")
	}
	if len(info.LinkedAccounts) != 2 {
		t.Fatalf("expected 2 linked accounts, got %d", len(info.LinkedAccounts))
	}
	if info.LinkedAccounts[0].ID != "222222222222" {
		t.Errorf("account[0].ID = %q, want %q", info.LinkedAccounts[0].ID, "222222222222")
	}
	if info.LinkedAccounts[0].Label != "dev-account" {
		t.Errorf("account[0].Label = %q, want %q", info.LinkedAccounts[0].Label, "dev-account")
	}
	if info.LinkedAccounts[1].ID != "333333333333" {
		t.Errorf("account[1].ID = %q, want %q", info.LinkedAccounts[1].ID, "333333333333")
	}
}

func TestDetectMonitoring_LinksError(t *testing.T) {
	mock := &mockOAMAPI{
		listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
			return &oam.ListSinksOutput{
				Items: []oamtypes.ListSinksItem{
					{Arn: aws.String("arn:aws:oam:us-east-1:111111111111:sink/sink-id")},
				},
			}, nil
		},
		listAttachedLinksFn: func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
			return nil, errors.New("throttled")
		},
	}

	info := detectMonitoring(context.Background(), mock)
	if info.IsMonitoring {
		t.Error("expected IsMonitoring to be false when links fail")
	}
}

func TestDetectMonitoring_Pagination(t *testing.T) {
	callCount := 0
	mock := &mockOAMAPI{
		listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
			return &oam.ListSinksOutput{
				Items: []oamtypes.ListSinksItem{
					{Arn: aws.String("arn:aws:oam:us-east-1:111111111111:sink/sink-id")},
				},
			}, nil
		},
		listAttachedLinksFn: func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
			callCount++
			if callCount == 1 {
				return &oam.ListAttachedLinksOutput{
					Items: []oamtypes.ListAttachedLinksItem{
						{
							LinkArn: aws.String("arn:aws:oam:us-east-1:222222222222:link/link-1"),
							Label:   aws.String("dev"),
						},
					},
					NextToken: aws.String("page2"),
				}, nil
			}
			return &oam.ListAttachedLinksOutput{
				Items: []oamtypes.ListAttachedLinksItem{
					{
						LinkArn: aws.String("arn:aws:oam:us-east-1:333333333333:link/link-2"),
						Label:   aws.String("staging"),
					},
				},
			}, nil
		},
	}

	info := detectMonitoring(context.Background(), mock)
	if !info.IsMonitoring {
		t.Fatal("expected IsMonitoring to be true")
	}
	if len(info.LinkedAccounts) != 2 {
		t.Fatalf("expected 2 linked accounts, got %d", len(info.LinkedAccounts))
	}
	if callCount != 2 {
		t.Errorf("expected 2 ListAttachedLinks calls, got %d", callCount)
	}
}

func TestExtractAccountFromLinkARN(t *testing.T) {
	tests := []struct {
		arn  string
		want string
	}{
		{"arn:aws:oam:us-east-1:123456789012:link/link-id", "123456789012"},
		{"arn:aws:oam:eu-west-1:987654321098:link/abc", "987654321098"},
		{"invalid", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractAccountFromLinkARN(tt.arn)
		if got != tt.want {
			t.Errorf("extractAccountFromLinkARN(%q) = %q, want %q", tt.arn, got, tt.want)
		}
	}
}
