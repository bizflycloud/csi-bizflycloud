
package driver

import (
	// "fmt"

	// "github.com/golang/protobuf/ptypes"

	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/cloud-provider-openstack/pkg/volume/util"
	"k8s.io/klog"

	// "github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	// ossnapshots "github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	// "github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	// "k8s.io/cloud-provider-openstack/pkg/csi/cinder/openstack"
	// cpoerrors "k8s.io/cloud-provider-openstack/pkg/util/errors"
	// "k8s.io/cloud-provider-openstack/pkg/volume/util"
	"github.com/bizflycloud/gobizfly"

	// "k8s.io/klog"
)

type controllerServer struct {
	Driver *VolumeDriver
	Client	*gobizfly.Client
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Volume Name
	volName := req.GetName()

	if len(volName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume request missing Volume Name")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume request missing Volume capability")
	}

	// Volume Size - Default is 1 GiB
	volSizeBytes := int64(1 * 1024 * 1024 * 1024)
	if req.GetCapacityRange() != nil {
		volSizeBytes = int64(req.GetCapacityRange().GetRequiredBytes())
	}
	volSizeGB := int(util.RoundUpSize(volSizeBytes, 1024*1024*1024))

	// Volume Type
	volType := req.GetParameters()["type"]

	var volAvailability string
	if req.GetAccessibilityRequirements() != nil {
		volAvailability = getAZFromTopology(req.GetAccessibilityRequirements())
	}

	if len(volAvailability) == 0 {
		// Volume Availability - Default is nova
		volAvailability = req.GetParameters()["availability"]
	}

	client := cs.Client

	// Verify a volume with the provided name doesn't already exist for this tenant
	volume, err := GetVolumesByName(ctx, client, volName)
	if err != nil {
		klog.V(3).Infof("Failed to query for existing Volume during CreateVolume: %v", err)
	}

	// if volume {
	if volume {
		if volSizeGB != volume.Size {
			return nil, status.Error(codes.AlreadyExists, "Volume Already exists with same name and different capacity")
		}
	
		klog.V(4).Infof("Volume %s already exists in Availability Zone: %s of size %d GiB", volume.ID, volume.AvailabilityZone, volume.Size)
		return getCreateVolumeResponse(&volume), nil	
	}


	// Volume Create
	//properties := map[string]string{"cinder.csi.openstack.org/cluster": cs.Driver.cluster}
	content := req.GetVolumeContentSource()
	var snapshotID string
	//var sourceVolID string

	if content != nil && content.GetSnapshot() != nil {
		snapshotID = content.GetSnapshot().GetSnapshotId()
	}

	//if content != nil && content.GetVolume() != nil {
	//	sourceVolID = content.GetVolume().GetVolumeId()
	//}

	//vol, err := cloud.CreateVolume(volName, volSizeGB, volType, volAvailability, snapshotID, sourcevolID, &properties)
	vcr := gobizfly.VolumeCreateRequest{
		Name: volName,
		Size: volSizeGB,
		VolumeType: volType,
		AvailabilityZone: volAvailability,
		SnapshotID: snapshotID,
	}
	vol, err := cs.Client.Volume.Create(ctx, &vcr)
	if err != nil {
		klog.V(3).Infof("Failed to CreateVolume: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("CreateVolume failed with error %v", err))

	}

	klog.V(4).Infof("Create volume %s in Availability Zone: %s of size %d GiB", vol.ID, vol.AvailabilityZone, vol.Size)

	return getCreateVolumeResponse(vol), nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, nil
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, nil
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {

	return nil, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {

	return nil, nil
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return nil, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

	return nil, nil
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, nil
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {

	return nil, nil
}

// func getAZFromTopology(requirement *csi.TopologyRequirement) string {
// 	for _, topology := range requirement.GetPreferred() {
// 		zone, exists := topology.GetSegments()[topologyKey]
// 		if exists {
// 			return zone
// 		}
// 	}

// 	for _, topology := range requirement.GetRequisite() {
// 		zone, exists := topology.GetSegments()[topologyKey]
// 		if exists {
// 			return zone
// 		}
// 	}
// 	return ""
// }

// func getCreateVolumeResponse(vol *volumes.Volume) *csi.CreateVolumeResponse {

// 	var volsrc *csi.VolumeContentSource

// 	if vol.SnapshotID != "" {
// 		volsrc = &csi.VolumeContentSource{
// 			Type: &csi.VolumeContentSource_Snapshot{
// 				Snapshot: &csi.VolumeContentSource_SnapshotSource{
// 					SnapshotId: vol.SnapshotID,
// 				},
// 			},
// 		}
// 	}

// 	if vol.SourceVolID != "" {
// 		volsrc = &csi.VolumeContentSource{
// 			Type: &csi.VolumeContentSource_Volume{
// 				Volume: &csi.VolumeContentSource_VolumeSource{
// 					VolumeId: vol.SourceVolID,
// 				},
// 			},
// 		}
// 	}

// 	resp := &csi.CreateVolumeResponse{
// 		Volume: &csi.Volume{
// 			VolumeId:      vol.ID,
// 			CapacityBytes: int64(vol.Size * 1024 * 1024 * 1024),
// 			AccessibleTopology: []*csi.Topology{
// 				{
// 					Segments: map[string]string{topologyKey: vol.AvailabilityZone},
// 				},
// 			},
// 			ContentSource: volsrc,
// 		},
// 	}

// 	return resp

// }
