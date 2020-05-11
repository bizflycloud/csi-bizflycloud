package driver

import (
	"errors"
	"fmt"

	"github.com/bizflycloud/gobizfly"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/cloud-provider-openstack/pkg/volume/util"
	"k8s.io/klog"
)

type controllerServer struct {
	Driver *VolumeDriver
	Client *gobizfly.Client
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
	volCategory := req.GetParameters()["category"]

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
	if volume != nil {
		if volSizeGB != volume.Size {
			return nil, status.Error(codes.AlreadyExists, "Volume Already exists with same name and different capacity")
		}

		klog.V(4).Infof("Volume %s already exists in Availability Zone: %s of size %d GiB", volume.ID, volume.AvailabilityZone, volume.Size)
		return getCreateVolumeResponse(volume), nil
	}

	// Volume Create
	content := req.GetVolumeContentSource()
	var snapshotID string

	if content != nil && content.GetSnapshot() != nil {
		snapshotID = content.GetSnapshot().GetSnapshotId()
	}

	vcr := gobizfly.VolumeCreateRequest{
		Name:             volName,
		Size:             volSizeGB,
		VolumeType:       volType,
		AvailabilityZone: volAvailability,
		SnapshotID:       snapshotID,
		VolumeCategory:   volCategory,
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
	// Volume Delete
	volID := req.GetVolumeId() // Volume name
	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	err := cs.Client.Volume.Delete(ctx, volID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(3).Infof("Volume %s is already deleted.", volID)
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.V(3).Infof("Failed to DeleteVolume: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("DeleteVolume failed with error %v", err))
	}

	klog.V(4).Infof("Delete volume %s", volID)

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	// Volume Attach
	instanceID := req.GetNodeId()
	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	if len(instanceID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Instance ID must be provided")
	}

	_, err := cs.Client.Volume.Get(ctx, volumeID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "ControllerPublishVolume Volume not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerPublishVolume get volume failed with error %v", err))
	}

	svr, err := cs.Client.Server.Get(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "ControllerPublishVolume Instance not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerPublishVolume GetInstanceByID failed with error %v", err))
	}

	// Check volume is already attached
	if volumeInServer(volumeID, svr.AttachedVolumes) {
		goto publishVolume
	}

	_, err = cs.Client.Volume.Attach(ctx, volumeID, instanceID)
	if err != nil {
		klog.V(3).Infof("Failed to AttachVolume: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerPublishVolume Attach Volume failed with error %v", err))

	}
	err = WaitDiskAttached(ctx, cs.Client, instanceID, volumeID)
	if err != nil {
		klog.V(3).Infof("Failed to WaitDiskAttached: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerPublishVolume failed with error %v", err))
	}
publishVolume:
	devicePath, err := GetAttachmentDiskPath(ctx, cs.Client, instanceID, volumeID)
	if err != nil {
		klog.V(3).Infof("Failed to GetAttachmentDiskPath: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerPublishVolume failed with error %v", err))
	}

	klog.V(4).Infof("ControllerPublishVolume %s on %s", volumeID, instanceID)

	// Publish Volume Info
	pvInfo := map[string]string{}
	pvInfo["DevicePath"] = devicePath

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: pvInfo,
	}, nil
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	// Volume Detach
	instanceID := req.GetNodeId()
	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnpublishVolume Volume ID must be provided")
	}
	server, err := cs.Client.Server.Get(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(3).Infof("ControllerUnpublishVolume assuming volume %s is detached, because node %s does not exist", volumeID, instanceID)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerUnpublishVolume GetInstanceByID failed with error %v", err))
	}

	if !volumeInServer(volumeID, server.AttachedVolumes) {
		klog.V(3).Infof("ControllerUnpublishVolume assuming volume %s is detached", volumeID)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	_, err = cs.Client.Volume.Detach(ctx, volumeID, instanceID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(3).Infof("ControllerUnpublishVolume assuming volume %s is detached, because it does not exist", volumeID)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		klog.V(3).Infof("Failed to DetachVolume: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerUnpublishVolume Detach Volume failed with error %v", err))
	}

	err = WaitDiskDetached(ctx, cs.Client, instanceID, volumeID)
	if err != nil {
		klog.V(3).Infof("Failed to WaitDiskDetached: %v", err)
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(3).Infof("ControllerUnpublishVolume assuming volume %s is detached, because it was deleted in the meanwhile", volumeID)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("ControllerUnpublishVolume failed with error %v", err))
	}

	klog.V(4).Infof("ControllerUnpublishVolume %s on %s", volumeID, instanceID)

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	vlist, err := cs.Client.Volume.List(ctx, &gobizfly.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Failed to ListVolumes: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("ListVolumes failed with error %v", err))
	}

	var ventries []*csi.ListVolumesResponse_Entry
	for _, v := range vlist {
		ventry := csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      v.ID,
				CapacityBytes: int64(v.Size * 1024 * 1024 * 1024),
			},
		}
		ventries = append(ventries, &ventry)
	}
	return &csi.ListVolumesResponse{
		Entries: ventries,
	}, nil
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	name := req.Name
	volumeId := req.SourceVolumeId

	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot name must be provided in CreateSnapshot request")
	}

	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeID must be provided in CreateSnapshot request")
	}

	// Verify a snapshot with the provided name doesn't already exist for this tenant
	snapshots, err := GetSnapshotByNameAndVolumeID(ctx, cs.Client, volumeId, name)
	if err != nil {
		klog.V(3).Infof("Failed to query for existing Snapshot during CreateSnapshot: %v", err)
	}
	var snap *gobizfly.Snapshot

	if len(snapshots) == 1 {
		snap = snapshots[0]

		if snap.VolumeId != volumeId {
			return nil, status.Error(codes.AlreadyExists, "Snapshot with given name already exists, with different source volume ID")
		}

		klog.V(3).Infof("Found existing snapshot %s on %s", name, volumeId)

	} else if len(snapshots) > 1 {
		klog.V(3).Infof("found multiple existing snapshots with selected name (%s) during create", name)
		return nil, status.Error(codes.Internal, "Multiple snapshots reported by Cinder with same name")

	} else {
		scr := gobizfly.SnapshotCreateRequest{
			Name:     name,
			VolumeId: volumeId,
			Force:    true,
		}
		snap, err = cs.Client.Snapshot.Create(ctx, &scr)
		if err != nil {
			klog.V(3).Infof("Failed to Create snapshot: %v", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("CreateSnapshot failed with error %v", err))
		}

		klog.V(3).Infof("CreateSnapshot %s on %s", name, volumeId)
	}

	//TODO Convert Create time
	//ctime, err := ptypes.TimestampProto(snap.CreateAt)
	//if err != nil {
	//	klog.Errorf("Error to convert time to timestamp: %v", err)
	//}

	err = WaitSnapshotReady(ctx, cs.Client, snap.Id)
	if err != nil {
		klog.V(3).Infof("Failed to WaitSnapshotReady: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("CreateSnapshot failed with error %v", err))
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snap.Id,
			SizeBytes:      int64(snap.Size * 1024 * 1024 * 1024),
			SourceVolumeId: snap.VolumeId,
			//CreationTime:   ctime,
			ReadyToUse: true,
		},
	}, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {

	id := req.SnapshotId

	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID must be provided in DeleteSnapshot request")
	}

	err := cs.Client.Snapshot.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(3).Infof("Snapshot %s is already deleted.", id)
			return &csi.DeleteSnapshotResponse{}, nil
		}
		klog.V(3).Infof("Failed to Delete snapshot: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("DeleteSnapshot failed with error %v", err))
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {

	if len(req.GetSnapshotId()) != 0 {
		snap, err := cs.Client.Snapshot.Get(ctx, req.SnapshotId)
		if err != nil {
			klog.V(3).Infof("Failed to Get snapshot: %v", err)
			return &csi.ListSnapshotsResponse{}, nil
		}

		//TODO Convert Create time
		//ctime, err := ptypes.TimestampProto(snap.CreateAt)

		entry := &csi.ListSnapshotsResponse_Entry{
			Snapshot: &csi.Snapshot{
				SizeBytes:      int64(snap.Size * 1024 * 1024 * 1024),
				SnapshotId:     snap.Id,
				SourceVolumeId: snap.VolumeId,
				//CreationTime:   ctime,
				ReadyToUse: true,
			},
		}

		entries := []*csi.ListSnapshotsResponse_Entry{entry}
		return &csi.ListSnapshotsResponse{
			Entries: entries,
		}, err

	}

	var vlist []*gobizfly.Snapshot
	var err error

	if len(req.GetSourceVolumeId()) != 0 {
		vlist, err = GetSnapshotByNameAndVolumeID(ctx, cs.Client, "", req.GetSourceVolumeId())
		if err != nil {
			klog.V(3).Infof("Failed to ListSnapshots: %v", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("ListSnapshots get snapshot failed with error %v", err))
		}
	} else {
		vlist, err = cs.Client.Snapshot.List(ctx, &gobizfly.ListOptions{})
		if err != nil {
			klog.V(3).Infof("Failed to ListSnapshots: %v", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("ListSnapshots get snapshot failed with error %v", err))

		}

	}

	var ventries []*csi.ListSnapshotsResponse_Entry
	for _, v := range vlist {
		//TODO Convert Create time
		//ctime, err := ptypes.TimestampProto(v.CreateAt)
		//if err != nil {
		//	klog.Errorf("Error to convert time to timestamp: %v", err)
		//}
		ventry := csi.ListSnapshotsResponse_Entry{
			Snapshot: &csi.Snapshot{
				SizeBytes:      int64(v.Size * 1024 * 1024 * 1024),
				SnapshotId:     v.Id,
				SourceVolumeId: v.VolumeId,
				//CreationTime:   ctime,
				ReadyToUse: true,
			},
		}
		ventries = append(ventries, &ventry)
	}
	return &csi.ListSnapshotsResponse{
		Entries: ventries,
	}, nil
}

// ControllerGetCapabilities implements the default GRPC callout.
// Default supports all capabilities
func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(5).Infof("Using default ControllerGetCapabilities")

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.Driver.cscap,
	}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	reqVolCap := req.GetVolumeCapabilities()

	if reqVolCap == nil || len(reqVolCap) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume Capabilities must be provided")
	}
	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume ID must be provided")
	}

	_, err := cs.Client.Volume.Get(ctx, volumeID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("ValidateVolumeCapabiltites Volume %s not found", volumeID))
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("ValidateVolumeCapabiltites %v", err))
	}

	for _, cap := range reqVolCap {
		if cap.GetAccessMode().GetMode() != cs.Driver.vcap[0].Mode {
			return &csi.ValidateVolumeCapabilitiesResponse{Message: "Requested Volume Capabilty not supported"}, nil
		}
	}

	// Cinder CSI driver currently supports one mode only
	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: cs.Driver.vcap[0],
				},
			},
		},
	}

	return resp, nil
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, fmt.Sprintf("GetCapacity is not yet implemented"))
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, nil
}

func getAZFromTopology(requirement *csi.TopologyRequirement) string {
	for _, topology := range requirement.GetPreferred() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}

	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[topologyKey]
		if exists {
			return zone
		}
	}
	return ""
}

func getCreateVolumeResponse(vol *gobizfly.Volume) *csi.CreateVolumeResponse {

	var volsrc *csi.VolumeContentSource

	if vol.SnapshotID != "" {
		volsrc = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: vol.SnapshotID,
				},
			},
		}
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.ID,
			CapacityBytes: int64(vol.Size * 1024 * 1024 * 1024),
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{topologyKey: vol.AvailabilityZone},
				},
			},
			ContentSource: volsrc,
		},
	}

	return resp

}

func volumeInServer(volId string, attachedVolumes []gobizfly.AttachedVolume) bool {
	for _, a := range attachedVolumes {
		if a.ID == volId {
			return true
		}
	}
	return false
}
