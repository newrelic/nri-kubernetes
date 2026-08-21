package kubelet

import "testing"

func TestWithCloudClusterID(t *testing.T) {
	t.Parallel()
	s := &Scraper{}
	if err := WithCloudClusterID("arn:aws:eks:us-east-1:123456789012:cluster/test")(s); err != nil {
		t.Fatalf("WithCloudClusterID() unexpected error: %v", err)
	}
	if got, want := s.cloudClusterID, "arn:aws:eks:us-east-1:123456789012:cluster/test"; got != want {
		t.Errorf("cloudClusterID = %q, want %q", got, want)
	}
}
