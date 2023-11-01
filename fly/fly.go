// Utilities to interact with Fly.io platform
package fly

import (
	"context"
	"fmt"
	"net"
	"time"
)

type VMInfo struct {
	ID     string
	Region string
}

// ClusterInfo contains information about the Fly.io cluster
// obtained from the DNS records, such as the number of machines and regions
type ClusterInfo struct {
	regions []string
	vms     []*VMInfo
}

func (c *ClusterInfo) NumRegions() int {
	return len(c.regions)
}

func (c *ClusterInfo) Regions() []string {
	return c.regions
}

func (c *ClusterInfo) VMs() []string {
	ids := make([]string, len(c.vms))
	for i, vm := range c.vms {
		ids[i] = vm.ID
	}
	return ids
}

func (c *ClusterInfo) NumVMs() int {
	return len(c.vms)
}

func Cluster(appName string) (*ClusterInfo, error) {
	addr := fmt.Sprintf("vms.%s.internal.", appName)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	textRecords, err := net.DefaultResolver.LookupTXT(ctx, addr)

	if err != nil {
		return nil, err
	}

	vms := make([]*VMInfo, len(textRecords))
	regionsMap := make(map[string]struct{})
	regions := make([]string, 0)

	for i, txt := range textRecords {
		vm := &VMInfo{}
		fmt.Sscanf(txt, "%s %s", &vm.ID, &vm.Region)
		vms[i] = vm

		if _, ok := regionsMap[vm.Region]; !ok {
			regionsMap[vm.Region] = struct{}{}
			regions = append(regions, vm.Region)
		}
	}

	return &ClusterInfo{regions: regions, vms: vms}, nil
}
