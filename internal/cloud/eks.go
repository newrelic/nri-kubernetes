package cloud

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

var (
	errIncompleteEKSMetadata  = errors.New("incomplete EKS metadata")
	errMissingRegionInstance  = errors.New("missing region/instance-id from providerID")
	errClusterNameTagNotFound = errors.New("cluster-name tag not found on instance")
)

// eksClusterNameTag is the tag EKS automatically applies to member EC2 instances.
const eksClusterNameTag = "aws:eks:cluster-name"

type AwsPartition string

const (
	DefaultPartition AwsPartition = "aws"
	GovPartition     AwsPartition = "aws-us-gov"
	ChinaPartition   AwsPartition = "aws-cn"
)

// eksDetector resolves the EKS cluster ARN via the EC2 API (IMDS does not expose the
// cluster name), using Pod Identity / IRSA credentials.
type eksDetector struct {
	newEC2Client func(ctx context.Context, region string) (ec2DescribeInstancesAPI, error)
}

// ec2DescribeInstancesAPI is the subset of the EC2 client used here, for test injection.
type ec2DescribeInstancesAPI interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// defaultNewEC2Client builds an EC2 client from the default credential chain (which
// picks up IRSA web-identity tokens and the Pod Identity endpoint automatically).
func defaultNewEC2Client(ctx context.Context, region string) (ec2DescribeInstancesAPI, error) { //nolint:ireturn // Returning interface is correct design for abstraction.
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return ec2.NewFromConfig(cfg), nil
}

// detect assembles arn:<partition>:eks:<region>:<accountId>:cluster/<clusterName>.
func (e eksDetector) detect(ctx context.Context, providerID string) (string, error) {
	availabilityZone, instanceID := parseAWSProviderID(providerID)
	region := azToRegion(availabilityZone)

	clusterName, accountID, err := e.identityFromAPI(ctx, region, instanceID)
	if err != nil {
		return "", fmt.Errorf("EKS detection failed (API): %w", err)
	}
	if region == "" || accountID == "" || clusterName == "" {
		return "", fmt.Errorf("%w: region=%q account=%q cluster=%q", errIncompleteEKSMetadata, region, accountID, clusterName)
	}
	return fmt.Sprintf("arn:%s:eks:%s:%s:cluster/%s", awsPartition(region), region, accountID, clusterName), nil
}

// identityFromAPI reads the cluster name (instance tag) and account id (reservation owner) via the EC2 API.
func (e eksDetector) identityFromAPI(ctx context.Context, region, instanceID string) (clusterName, accountID string, err error) {
	if region == "" || instanceID == "" {
		return "", "", errMissingRegionInstance
	}
	client, err := e.newEC2Client(ctx, region)
	if err != nil {
		return "", "", err
	}
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		return "", "", fmt.Errorf("describing instance %s: %w", instanceID, err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			for _, tag := range inst.Tags {
				if aws.ToString(tag.Key) == eksClusterNameTag {
					return aws.ToString(tag.Value), aws.ToString(r.OwnerId), nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("%w: tag %s instance %s", errClusterNameTagNotFound, eksClusterNameTag, instanceID)
}

// parseAWSProviderID extracts availability zone and instance-id from aws:///<az>/<instance-id>.
func parseAWSProviderID(providerID string) (availabilityZone, instanceID string) {
	re := regexp.MustCompile(`^aws:///([a-zA-Z0-9._\-()]+)/([a-zA-Z0-9._\-()]+)$`)
	matches := re.FindStringSubmatch(providerID)
	if matches == nil {
		return "", ""
	}
	return matches[1], matches[2]
}

// azToRegion strips the trailing AZ letter from an availability zone (us-east-1a -> us-east-1).
func azToRegion(az string) string {
	if az == "" {
		return ""
	}
	if last := az[len(az)-1]; last >= 'a' && last <= 'z' {
		return az[:len(az)-1]
	}
	return az
}

// awsPartition returns the ARN partition for a region.
func awsPartition(region string) AwsPartition {
	switch {
	case strings.HasPrefix(region, "us-gov-"):
		return GovPartition
	case strings.HasPrefix(region, "cn-"):
		return ChinaPartition
	default:
		return DefaultPartition
	}
}
