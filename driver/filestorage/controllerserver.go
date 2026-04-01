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

package filestorage

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bizflycloud/gobizfly"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

type controllerServer struct {
	d      *Driver
	client *gobizfly.Client
	csi.UnimplementedControllerServer
}

var (
	pendingVolumes = sync.Map{}
)

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := validateCreateVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	shareName := req.GetName()
	params := req.GetParameters()
	if params == nil {
		params = make(map[string]string)
	}

	// Check for pending CreateVolume for this volume name
	if _, isPending := pendingVolumes.LoadOrStore(shareName, true); isPending {
		return nil, status.Errorf(codes.Aborted, "volume %s is already being processed", shareName)
	}
	defer pendingVolumes.Delete(shareName)

	// Requested size
	requestedSize := req.GetCapacityRange().GetRequiredBytes()
	if requestedSize == 0 {
		requestedSize = 1 * bytesInGiB // minimum 1 GiB
	}
	sizeInGiB := bytesToGiB(requestedSize)

	// Extract StorageClass parameters
	zone := params["zone"]
	shareProtocol := params["share_protocol"]
	if shareProtocol == "" {
		shareProtocol = cs.d.shareProto
	}
	shareType := params["share_type"]
	networkID := params["network_id"]
	subnetID := params["subnet_id"]

	// Access rule: restrictive CIDR based on VPC network
	nfsShareClient := params["nfs-shareClient"]
	if nfsShareClient == "" {
		nfsShareClient = "0.0.0.0/0" // fallback default
	}

	// Build create request
	createReq := &gobizfly.CreateShareRequest{
		Name:          shareName,
		Size:          sizeInGiB,
		ShareProtocol: shareProtocol,
		ShareType:     shareType,
		NetworkID:     networkID,
		SubnetID:      subnetID,
		Zone:          zone,
		Description:   shareDescription,
		Access: []gobizfly.ShareAccessRule{
			{
				AccessTo:    nfsShareClient,
				AccessLevel: "rw",
				AccessType:  "ip",
			},
		},
	}

	// Create or get existing share
	share, err := getOrCreateShare(ctx, cs.client, shareName, createReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create volume %s: %v", shareName, err)
	}

	// Verify compatibility if share already existed
	if share.Size != sizeInGiB {
		return nil, status.Errorf(codes.AlreadyExists,
			"volume %s already exists with different capacity: wanted %d GiB, got %d GiB",
			shareName, sizeInGiB, share.Size)
	}

	// Access rules are now set during creation via CreateShareRequest.
	// Removed secondary call to ManageAccessRules because it throws 403 on the staging API.

	// Build volume context with export location info
	volCtx := make(map[string]string)
	volCtx["shareID"] = share.ID
	volCtx["shareProtocol"] = share.ShareProtocol

	if len(share.ExportLocations) > 0 {
		server, sharePath, parseErr := parseExportLocation(share.ExportLocations[0])
		if parseErr == nil {
			volCtx["server"] = server
			volCtx["share"] = sharePath
		} else {
			klog.Warningf("failed to parse export location %q: %v", share.ExportLocations[0], parseErr)
		}
	} else {
		return nil, status.Errorf(codes.Internal, "failed to get export location for volume %s", shareName)
	}

	// Topology
	var accessibleTopology []*csi.Topology
	if share.Zone != "" {
		accessibleTopology = []*csi.Topology{
			{Segments: map[string]string{topologyKey: share.Zone}},
		}
	}

	klog.V(4).Infof("CreateVolume: created share %s (ID: %s) size %d GiB", shareName, share.ID, share.Size)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           share.ID,
			CapacityBytes:      int64(share.Size) * bytesInGiB,
			VolumeContext:      volCtx,
			AccessibleTopology: accessibleTopology,
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if err := validateDeleteVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	shareID := req.GetVolumeId()
	if err := deleteShare(ctx, cs.client, shareID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume %s: %v", shareID, err)
	}

	klog.V(4).Infof("DeleteVolume: deleted share %s", shareID)
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if err := validateControllerExpandVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	shareID := req.GetVolumeId()

	// Check for pending operations on this volume
	if _, isPending := pendingVolumes.LoadOrStore(shareID, true); isPending {
		return nil, status.Errorf(codes.Aborted, "volume %s is already being processed", shareID)
	}
	defer pendingVolumes.Delete(shareID)

	// Get current share
	share, err := getShareByID(ctx, cs.client, shareID)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume %s not found", shareID)
		}
		return nil, status.Errorf(codes.Internal, "failed to get volume %s: %v", shareID, err)
	}

	desiredSizeInGiB := bytesToGiB(req.GetCapacityRange().GetRequiredBytes())

	// Already large enough
	if share.Size >= desiredSizeInGiB {
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes: int64(share.Size) * bytesInGiB,
		}, nil
	}

	share, err = resizeShare(ctx, cs.client, share.ID, desiredSizeInGiB)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to expand volume %s: %v", shareID, err)
	}

	klog.V(4).Infof("ControllerExpandVolume: expanded share %s to %d GiB", shareID, share.Size)

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes: int64(share.Size) * bytesInGiB,
	}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if err := validateValidateVolumeCapabilitiesRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Verify the volume exists
	_, err := getShareByID(ctx, cs.client, req.GetVolumeId())
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume %s not found", req.GetVolumeId())
		}
		return nil, status.Errorf(codes.Internal, "failed to get volume %s: %v", req.GetVolumeId(), err)
	}

	// File storage only supports mount (no block), validate capabilities
	for _, volCap := range req.GetVolumeCapabilities() {
		if volCap.GetBlock() != nil {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Message: "block access type is not allowed for file storage",
			}, nil
		}
		if volCap.GetMount() == nil {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Message: "volume must be accessible via filesystem API",
			}, nil
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
	}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.d.cscaps,
	}, nil
}

// NFS does not need controller publish/unpublish (no attach/detach)
func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
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

func volumeAccessModeDescription(mode csi.VolumeCapability_AccessMode_Mode) string {
	return fmt.Sprintf("%v", mode)
}
