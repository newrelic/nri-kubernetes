package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	log "github.com/sirupsen/logrus"
)

// eksClusterNameTag is the tag EKS automatically applies to member EC2 instances.
const eksClusterNameTag = "aws:eks:cluster-name"

// detectEKS resolves the cluster name, preferring IMDS instance tags (no AWS
// credentials required) and falling back to the EC2 API using Pod Identity / IRSA
// credentials when IMDS is unavailable or metadata tags are not enabled.
func detectEKS(ctx context.Context, logger *log.Logger, region, instanceID string) (string, error) {
	if name, err := eksClusterNameFromIMDS(ctx); err == nil && name != "" {
		return name, nil
	} else if err != nil {
		logger.Debugf("cloud: EKS IMDS tag lookup failed, trying AWS API: %v", err)
	}

	name, err := eksClusterNameFromAPI(ctx, region, instanceID)
	if err != nil {
		return "", fmt.Errorf("EKS detection failed (IMDS and API): %w", err)
	}
	return name, nil
}

func eksClusterNameFromIMDS(ctx context.Context) (string, error) {
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPut, awsIMDSBaseURL+"/api/token", nil)
	if err != nil {
		return "", fmt.Errorf("building IMDS token request: %w", err)
	}
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	token, err := doMetadataGet(tokenReq)
	if err != nil {
		return "", fmt.Errorf("getting IMDSv2 token: %w", err)
	}

	tagReq, err := http.NewRequestWithContext(ctx, http.MethodGet, awsIMDSBaseURL+"/meta-data/tags/instance/"+eksClusterNameTag, nil)
	if err != nil {
		return "", fmt.Errorf("building IMDS tag request: %w", err)
	}
	tagReq.Header.Set("X-aws-ec2-metadata-token", token)
	name, err := doMetadataGet(tagReq)
	if err != nil {
		return "", fmt.Errorf("reading %s tag from IMDS: %w", eksClusterNameTag, err)
	}
	return strings.TrimSpace(name), nil
}

// ec2DescribeInstancesAPI is the subset of the EC2 client used here, for test injection.
type ec2DescribeInstancesAPI interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// newEC2Client builds an EC2 client from the default credential chain (which picks
// up IRSA web-identity tokens and the Pod Identity endpoint automatically).
var newEC2Client = func(ctx context.Context, region string) (ec2DescribeInstancesAPI, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return ec2.NewFromConfig(cfg), nil
}

func eksClusterNameFromAPI(ctx context.Context, region, instanceID string) (string, error) {
	if region == "" || instanceID == "" {
		return "", fmt.Errorf("missing region/instance-id from providerID")
	}
	client, err := newEC2Client(ctx, region)
	if err != nil {
		return "", err
	}
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		return "", fmt.Errorf("describing instance %s: %w", instanceID, err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			for _, tag := range inst.Tags {
				if aws.ToString(tag.Key) == eksClusterNameTag {
					return aws.ToString(tag.Value), nil
				}
			}
		}
	}
	return "", fmt.Errorf("tag %s not found on instance %s", eksClusterNameTag, instanceID)
}
