package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/bizflycloud/gobizfly"
	"k8s.io/cloud-provider-openstack/pkg/csi/cinder/openstack"
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
	name        string
	nodeID      string
	version     string
	endpoint    string
	cluster     string

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
			//csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
			//csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
		})
	d.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER})

	d.AddNodeServiceCapabilities(
		[]csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			//csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
			//csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		})

	return d
}

// AddControllerServiceCapabilities add capabilities for driver
func (d *VolumeDriver) AddControllerServiceCapabilities(cl []csi.ControllerServiceCapability_RPC_Type) {
	var csc []*csi.ControllerServiceCapability

	for _, c := range cl {
		klog.Infof("Enabling controller service capability: %v", c.String())
		csc = append(csc, NewControllerServiceCapability(c))
	}

	d.cscap = csc

	return
}

// AddVolumeCapabilityAccessModes add access mode capability for volume driver
func (d *VolumeDriver) AddVolumeCapabilityAccessModes(vc []csi.VolumeCapability_AccessMode_Mode) []*csi.VolumeCapability_AccessMode {
	var vca []*csi.VolumeCapability_AccessMode
	for _, c := range vc {
		klog.Infof("Enabling volume access mode: %v", c.String())
		vca = append(vca, NewVolumeCapabilityAccessMode(c))
	}
	d.vcap = vca
	return vca
}

// AddNodeServiceCapabilities add node service capabilities
func (d *VolumeDriver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) error {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		klog.Infof("Enabling node service capability: %v", n.String())
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	d.nscap = nsc
	return nil
}

// SetupDriver setups driver for volume driver
func (d *VolumeDriver) SetupDriver(client *gobizfly.Client, mount mount.IMount, metadata openstack.IMetadata) {

	d.ids = NewIdentityServer(d)
	d.cs = NewControllerServer(d, client)
	d.ns = NewNodeServer(d, mount, metadata, client)

}

// Run run driver
func (d *VolumeDriver) Run() {

	RunControllerandNodePublishServer(d.endpoint, d.ids, d.cs, d.ns)
}

// RunControllerandNodePublishServer run controller
func RunControllerandNodePublishServer(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, ids, cs, ns)
	s.Wait()
}