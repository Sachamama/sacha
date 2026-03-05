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
	listSinksFn        func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error)
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

func TestDetectMonitoringAccount(t *testing.T) {
	tests := []struct {
		name         string
		mock         *mockOAMAPI
		wantMonitor  bool
		wantAccounts int
	}{
		{
			name: "not a monitoring account (no sinks)",
			mock: &mockOAMAPI{
				listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
					return &oam.ListSinksOutput{Items: []oamtypes.ListSinksItem{}}, nil
				},
			},
			wantMonitor:  false,
			wantAccounts: 0,
		},
		{
			name: "not a monitoring account (API error)",
			mock: &mockOAMAPI{
				listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
					return nil, errors.New("access denied")
				},
			},
			wantMonitor:  false,
			wantAccounts: 0,
		},
		{
			name: "monitoring account with linked accounts",
			mock: &mockOAMAPI{
				listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
					return &oam.ListSinksOutput{
						Items: []oamtypes.ListSinksItem{
							{Arn: aws.String("arn:aws:oam:us-east-1:111111111111:sink/abc123")},
						},
					}, nil
				},
				listAttachedLinksFn: func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
					return &oam.ListAttachedLinksOutput{
						Items: []oamtypes.ListAttachedLinksItem{
							{Label: aws.String("222222222222")},
							{Label: aws.String("333333333333")},
						},
					}, nil
				},
			},
			wantMonitor:  true,
			wantAccounts: 2,
		},
		{
			name: "monitoring account with link fetch error",
			mock: &mockOAMAPI{
				listSinksFn: func(ctx context.Context, params *oam.ListSinksInput, optFns ...func(*oam.Options)) (*oam.ListSinksOutput, error) {
					return &oam.ListSinksOutput{
						Items: []oamtypes.ListSinksItem{
							{Arn: aws.String("arn:aws:oam:us-east-1:111111111111:sink/abc123")},
						},
					}, nil
				},
				listAttachedLinksFn: func(ctx context.Context, params *oam.ListAttachedLinksInput, optFns ...func(*oam.Options)) (*oam.ListAttachedLinksOutput, error) {
					return nil, errors.New("access denied")
				},
			},
			wantMonitor:  true,
			wantAccounts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := detectMonitoringWithAPI(context.Background(), tt.mock)

			if info.IsMonitoring != tt.wantMonitor {
				t.Errorf("IsMonitoring = %v, want %v", info.IsMonitoring, tt.wantMonitor)
			}
			if len(info.LinkedAccounts) != tt.wantAccounts {
				t.Errorf("LinkedAccounts count = %d, want %d", len(info.LinkedAccounts), tt.wantAccounts)
			}
		})
	}
}
