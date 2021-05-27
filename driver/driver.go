// This file is part of csi-bizflycloud
//
// Copyright (C) 2020  BizFly Cloud
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>

package driver

import (
	"github.com/bizflycloud/gobizfly"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/cloud-provider-openstack/pkg/util/mount"
	"k8s.io/klog"
)

const (
	driverName  = "volume.csi.bizflycloud.vn"
	topologyKey = "topology." + driverName + "/zone"
)

var (
	version = "0.1"
)

type VolumeDriver struct {
	name     string
	nodeID   string
	version  string
	endpoint string
	cluster  string

	ids *identityServer
	cs  *controllerServer
	ns  *nodeServer

	vcap  []*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
	nscap []*csi.NodeServiceCapability
}

// NewDriver create new driver
func NewDriver(nodeID, endpoint, cluster string) *VolumeDriver {
	klog.Infof("Driver: %v version: %v", driverName, version)

	d := &VolumeDriver{}
	d.name = driverName
	d.nodeID = nodeID
	d.version = version
	d.endpoint = endpoint
	d.cluster = cluster

	d.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
			csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
			csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
			csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
		})
	d.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER})

	d.AddNodeServiceCapabilities(
		[]csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
			csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		})

	return d
}

// AddControllerServiceCapabilities add capabilities for driver
func (d *VolumeDriver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) {
	csc := make([]*csi.ControllerServiceCapability, 0, len(cl))
	for _, c := range cl {
		klog.Infof("Enabling controller service capability: %v", c.String())
		csc = append(csc, NewControllerServiceCapability(c))
	}
	d.cscap = csc
	return
}

// AddVolumeCapabilityAccessModes add access mode capability for volume driver
func (d *VolumeDriver) AddVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) []*csi.VolumeCapability_AccessMode {
	vca := make([]*csi.VolumeCapability_AccessMode, 0, len(vc))
	for _, c := range vc {
		klog.Infof("Enabling volume access mode: %v", c.String())
		vca = append(vca, NewVolumeCapabilityAccessMode(c))
	}
	d.vcap = vca
	return vca
}

// AddNodeServiceCapabilities add node service capabilities
func (d *VolumeDriver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) error {
	nsc := make([]*csi.NodeServiceCapability, 0, len(nl))
	for _, n := range nl {
		klog.Infof("Enabling node service capability: %v", n.String())
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	d.nscap = nsc
	return nil
}

// SetupControlDriver setups driver for control plane
func (d *VolumeDriver) SetupControlDriver(client *gobizfly.Client, mount mount.IMount, metadata metadata.IMetadata) {
	d.ids = NewIdentityServer(d)
	d.cs = NewControllerServer(d, client)
}

// SetupControlDriver setups driver for control plane
func (d *VolumeDriver) SetupNodeDriver(mount mount.IMount, metadata metadata.IMetadata) {
	d.ids = NewIdentityServer(d)
	d.ns = NewNodeServer(d, mount, metadata)
}

// Run run driver
func (d *VolumeDriver) Run() {
	RunControllerAndNodePublishServer(d.endpoint, d.ids, d.cs, d.ns)
}

// RunControllerAndNodePublishServer run controller
func RunControllerAndNodePublishServer(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, ids, cs, ns)
	s.Wait()
}
