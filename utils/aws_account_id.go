package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AwsAccountIdFromConfig returns the aws account id given an aws config.
// It makes a call to AWS Session Token Service's (STS) "GetCallerIdentity"
// API endpoint -- which does not require any IAM permissions to call.
func AwsAccountIdFromConfig(
	ctx context.Context,
	cfg aws.Config,
	timeout time.Duration,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stsClient := sts.NewFromConfig(cfg)

	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get AWS account ID via the AWS STS API: %w", err)
	}

	awsAccountId := aws.ToString(getCallerIdentityOutput.Account)
	if awsAccountId == "" {
		return "", fmt.Errorf("the AWS STS API returned an empty AWS account ID")
	}

	return awsAccountId, nil
}
