package fly

import (
	"testing"

	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster_multiple_nodes_and_regions(t *testing.T) {
	teardownDNS := mocks.MockDNSServer("vms.my-fly-app.internal.", []string{"xyz ewr", "abc sea", "def ewr"})
	defer teardownDNS()

	cluster, err := Cluster("my-fly-app")

	require.NoError(t, err)

	assert.Equal(t, 2, cluster.NumRegions())
	assert.Equal(t, 3, cluster.NumVMs())
	assert.EqualValues(t, []string{"ewr", "sea"}, cluster.Regions())
	assert.EqualValues(t, []string{"xyz", "abc", "def"}, cluster.VMs())
}

func TestCluster_when_dns_error(t *testing.T) {
	cluster, err := Cluster("my-fly-app")

	require.Error(t, err)
	assert.Nil(t, cluster)
}
