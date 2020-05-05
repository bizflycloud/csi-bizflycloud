package driver

import (
	// "fmt"
	// "os"
	// "path/filepath"
	// "strconv"
	// "strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	// "github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"golang.org/x/net/context"
	// "google.golang.org/grpc/codes"
	// "google.golang.org/grpc/status"
	"k8s.io/klog"
	// "k8s.io/kubernetes/pkg/util/resizefs"
	// utilpath "k8s.io/utils/path"

	"k8s.io/cloud-provider-openstack/pkg/csi/cinder/openstack"
	// "k8s.io/cloud-provider-openstack/pkg/util/blockdevice"
	// cpoerrors "k8s.io/cloud-provider-openstack/pkg/util/errors"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/cloud-provider-openstack/pkg/util/mount"
	"github.com/bizflycloud/gobizfly"
)

type nodeServer struct {
	Driver   *VolumeDriver
	Mount    mount.IMount
	Metadata openstack.IMetadata
	Client    *gobizfly.Client
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	return nil, nil
}

func nodePublishEphermeral(req *csi.NodePublishVolumeRequest, ns *nodeServer) (*csi.NodePublishVolumeResponse, error) {

	return nil, nil

}

func nodePublishVolumeForBlock(req *csi.NodePublishVolumeRequest, ns *nodeServer, mountOptions []string) (*csi.NodePublishVolumeResponse, error) {
	return nil, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return nil, nil

}

// func nodeUnpublishEphermeral(req *csi.NodeUnpublishVolumeRequest, ns *nodeServer, vol *volumes.Volume) (*csi.NodeUnpublishVolumeResponse, error) {
// 	volumeID := vol.ID
// 	var instanceID string

// 	if len(vol.Attachments) > 0 {
// 		instanceID = vol.Attachments[0].ServerID
// 	} else {
// 		return nil, status.Error(codes.FailedPrecondition, "Volume attachement not found in request")
// 	}

// 	err := ns.Cloud.DetachVolume(instanceID, volumeID)
// 	if err != nil {
// 		klog.V(3).Infof("Failed to DetachVolume: %v", err)
// 		return nil, status.Error(codes.Internal, err.Error())
// 	}

// 	err = ns.Cloud.WaitDiskDetached(instanceID, volumeID)
// 	if err != nil {
// 		klog.V(3).Infof("Failed to WaitDiskDetached: %v", err)
// 		return nil, status.Error(codes.Internal, err.Error())
// 	}

// 	err = ns.Cloud.DeleteVolume(volumeID)
// 	if err != nil {
// 		klog.V(3).Infof("Failed to DeleteVolume: %v", err)
// 		return nil, status.Error(codes.Internal, err.Error())
// 	}

// 	return &csi.NodeUnpublishVolumeResponse{}, nil
// }

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {

	return nil, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(5).Infof("NodeGetCapabilities called with req: %#v", req)

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.Driver.nscap,
	}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, nil
}

func getDevicePath(volumeID string, m mount.IMount) (string, error) {
	var devicePath string
	devicePath, _ = m.GetDevicePath(volumeID)
	if devicePath == "" {
		// try to get from metadata service
		devicePath = metadata.GetDevicePath(volumeID)
	}

	return devicePath, nil

}

// func getNodeIDMountProvider(m mount.IMount) (string, error) {
// 	nodeID, err := m.GetInstanceID()
// 	if err != nil {
// 		klog.V(3).Infof("Failed to GetInstanceID: %v", err)
// 		return "", err
// 	}
// 	klog.V(5).Infof("getNodeIDMountProvider return node id %s", nodeID)

// 	return nodeID, nil
// }

// func getNodeIDMetdataService(m openstack.IMetadata) (string, error) {

// 	nodeID, err := m.GetInstanceID()
// 	if err != nil {
// 		return "", err
// 	}
// 	klog.V(5).Infof("getNodeIDMetdataService return node id %s", nodeID)
// 	return nodeID, nil
// }

// func getAvailabilityZoneMetadataService(m openstack.IMetadata) (string, error) {

// 	zone, err := m.GetAvailabilityZone()
// 	if err != nil {
// 		return "", err
// 	}
// 	return zone, nil
// }

// func getNodeID(mount mount.IMount, iMetadata openstack.IMetadata, order string) (string, error) {
// 	elements := strings.Split(order, ",")

// 	var nodeID string
// 	var err error
// 	for _, id := range elements {
// 		id = strings.TrimSpace(id)
// 		switch id {
// 		case metadata.ConfigDriveID:
// 			nodeID, err = getNodeIDMountProvider(mount)
// 		case metadata.MetadataID:
// 			nodeID, err = getNodeIDMetdataService(iMetadata)
// 		default:
// 			err = fmt.Errorf("%s is not a valid metadata search order option. Supported options are %s and %s", id, metadata.ConfigDriveID, metadata.MetadataID)
// 		}
// 		if err == nil {
// 			break
// 		}
// 	}

// 	if err != nil {
// 		klog.Errorf("Failed to GetInstanceID from config drive or metadata service: %v", err)
// 		return "", err
// 	}
// 	return nodeID, nil
// }
