package cloud

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func Test_parseAWSProviderID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		providerID   string
		wantAZ       string
		wantInstance string
	}{
		{
			name:         "parseCorrectly", //nolint:goconst
			providerID:   "aws:///us-east-1a/i-0abc123def456",
			wantAZ:       "us-east-1a",
			wantInstance: "i-0abc123def456",
		},
		{
			name:         "emptyOnWrongPrefix",
			providerID:   "gce://my-project/us-central1-b/gke-node",
			wantAZ:       "",
			wantInstance: "",
		},
		{
			name:         "emptyOnTooFewSlashes",
			providerID:   "aws://us-east-1a/i-0abc",
			wantAZ:       "",
			wantInstance: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotAZ, gotInstance := parseAWSProviderID(tt.providerID)
			if gotAZ != tt.wantAZ || gotInstance != tt.wantInstance {
				t.Errorf("parseAWSProviderID() = %q, %q; want %q, %q", gotAZ, gotInstance, tt.wantAZ, tt.wantInstance)
			}
		})
	}
}

func Test_azToRegion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		az   string
		want string
	}{
		{"us-east-1a", "us-east-1"},
		{"us-gov-west-1a", "us-gov-west-1"},
		{"eu-central-1b", "eu-central-1"},
		{"us-east-2", "us-east-2"}, // already a region (trailing digit)
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.az, func(t *testing.T) {
			t.Parallel()
			if got := azToRegion(tt.az); got != tt.want {
				t.Errorf("azToRegion(%q) = %q, want %q", tt.az, got, tt.want)
			}
		})
	}
}

func Test_awsPartition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		region string
		want   AwsPartition
	}{
		{"us-east-1", DefaultPartition},
		{"eu-west-3", DefaultPartition},
		{"us-gov-west-1", GovPartition},
		{"cn-north-1", ChinaPartition},
	}
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			t.Parallel()
			if got := awsPartition(tt.region); got != tt.want {
				t.Errorf("awsPartition(%q) = %q, want %q", tt.region, got, tt.want)
			}
		})
	}
}

// stubEC2 is a test double for the EC2 DescribeInstances API.
type stubEC2 struct {
	out *ec2.DescribeInstancesOutput
	err error
}

func (s stubEC2) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return s.out, s.err
}

func reservationWithTags(ownerID string, tags map[string]string) []ec2types.Reservation {
	ec2Tags := make([]ec2types.Tag, 0, len(tags))
	for k, v := range tags {
		ec2Tags = append(ec2Tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return []ec2types.Reservation{{
		OwnerId:   aws.String(ownerID),
		Instances: []ec2types.Instance{{Tags: ec2Tags}},
	}}
}

func Test_detectEKS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		providerID   string
		reservations []ec2types.Reservation
		wantID       string
		wantErr      bool
	}{
		{
			name:         "standardPartition",
			providerID:   "aws:///us-east-1a/i-0abc",
			reservations: reservationWithTags("123456789012", map[string]string{eksClusterNameTag: "prod-cluster"}),
			wantID:       "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster",
		},
		{
			name:         "govPartition",
			providerID:   "aws:///us-gov-west-1a/i-0abc",
			reservations: reservationWithTags("111122223333", map[string]string{eksClusterNameTag: "gov-cluster"}),
			wantID:       "arn:aws-us-gov:eks:us-gov-west-1:111122223333:cluster/gov-cluster",
		},
		{
			name:         "chinaPartition",
			providerID:   "aws:///cn-north-1a/i-0abc",
			reservations: reservationWithTags("444455556666", map[string]string{eksClusterNameTag: "cn-cluster"}),
			wantID:       "arn:aws-cn:eks:cn-north-1:444455556666:cluster/cn-cluster",
		},
		{
			name:         "missingClusterTag",
			providerID:   "aws:///us-east-1a/i-0abc",
			reservations: reservationWithTags("123456789012", map[string]string{"Name": "some-node"}),
			wantErr:      true,
		},
		{
			name:       "invalidProviderID",
			providerID: "aws://bad",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := eksDetector{newEC2Client: func(context.Context, string) (ec2DescribeInstancesAPI, error) {
				return stubEC2{out: &ec2.DescribeInstancesOutput{Reservations: tt.reservations}}, nil
			}}

			got, err := d.detect(context.Background(), tt.providerID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("detectEKS() expected error, got id %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectEKS() unexpected error = %v", err)
			}
			if got != tt.wantID {
				t.Errorf("detectEKS() = %q, want %q", got, tt.wantID)
			}
		})
	}
}
