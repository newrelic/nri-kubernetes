package controlplane

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetEtcdTLSComponentOption(t *testing.T) {
	// first assert that ETCD has no TLS options by default
	components := BuildComponentList()
	etcd := findComponentByName(Etcd, components)

	assert.Equal(t, "", etcd.TLSSecretName)
	assert.Equal(t, "", etcd.TLSSecretNamespace)
	assert.True(t, etcd.Skip)

	// now set the TLS Configuration, and assert they are properly set
	const (
		tlsSecretName      = "my-secret-name"
		tlsSecretNamespace = "iluvtests"
	)

	components = BuildComponentList(WithEtcdTLSConfig(tlsSecretName, tlsSecretNamespace))
	etcd = findComponentByName(Etcd, components)

	assert.Equal(t, tlsSecretName, etcd.TLSSecretName)
	assert.Equal(t, tlsSecretNamespace, etcd.TLSSecretNamespace)
	assert.False(t, etcd.Skip)

}
