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
	"fmt"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/klog"
)

type nodeServer struct {
	d *Driver

	metadata          metadata.IMetadata
	supportsNodeStage bool

	// Cache for NodeStageVolume results, used by subsequent NodePublishVolume calls
	nodeStageCache    map[string]stageCacheEntry
	nodeStageCacheMtx sync.RWMutex

	csi.UnimplementedNodeServer
}

type stageCacheEntry struct {
	volumeContext map[string]string
}

// buildVolumeContext extracts the export location from volume context
// and builds the volume context map for the forwarded NFS CSI driver.
func (ns *nodeServer) buildVolumeContext(volumeContext map[string]string) (map[string]string, error) {
	// The controller already parsed the export location and put server/share in volume context
	server := volumeContext["server"]
	share := volumeContext["share"]

	if server == "" || share == "" {
		return nil, fmt.Errorf("volume context missing server or share fields")
	}

	return map[string]string{
		"server": server,
		"share":  share,
	}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if err := validateNodePublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volID := req.GetVolumeId()

	var (
		volumeCtx map[string]string
		err       error
	)

	if ns.supportsNodeStage {
		// Check stage cache first
		ns.nodeStageCacheMtx.RLock()
		cacheEntry, ok := ns.nodeStageCache[volID]
		ns.nodeStageCacheMtx.RUnlock()

		if ok {
			volumeCtx = cacheEntry.volumeContext
		} else {
			klog.Warningf("STAGE_UNSTAGE_VOLUME capability is enabled, but cache doesn't contain entry for %s - rebuilding...", volID)
			volumeCtx, err = ns.buildVolumeContext(req.GetVolumeContext())
		}
	} else {
		volumeCtx, err = ns.buildVolumeContext(req.GetVolumeContext())
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build volume context for volume %s: %v", volID, err)
	}

	// Forward the RPC to the NFS CSI node plugin
	csiConn, err := ns.d.csiClientBuilder.NewConnectionWithContext(ctx, ns.d.fwdEndpoint)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmtGrpcConnError(ns.d.fwdEndpoint, err))
	}
	defer csiConn.Close()

	req.VolumeContext = volumeCtx

	return ns.d.csiClientBuilder.NewNodeServiceClient(csiConn).PublishVolume(ctx, req)
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := validateNodeUnpublishVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	csiConn, err := ns.d.csiClientBuilder.NewConnectionWithContext(ctx, ns.d.fwdEndpoint)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmtGrpcConnError(ns.d.fwdEndpoint, err))
	}
	defer csiConn.Close()

	return ns.d.csiClientBuilder.NewNodeServiceClient(csiConn).UnpublishVolume(ctx, req)
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if err := validateNodeStageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volID := req.GetVolumeId()

	var (
		volumeCtx map[string]string
		err       error
	)

	ns.nodeStageCacheMtx.Lock()
	if cacheEntry, ok := ns.nodeStageCache[volID]; ok {
		volumeCtx = cacheEntry.volumeContext
	} else {
		volumeCtx, err = ns.buildVolumeContext(req.GetVolumeContext())
		if err == nil {
			ns.nodeStageCache[volID] = stageCacheEntry{volumeContext: volumeCtx}
		}
	}
	ns.nodeStageCacheMtx.Unlock()

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build volume context for volume %s: %v", volID, err)
	}

	// Forward the RPC to the NFS CSI node plugin
	csiConn, err := ns.d.csiClientBuilder.NewConnectionWithContext(ctx, ns.d.fwdEndpoint)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmtGrpcConnError(ns.d.fwdEndpoint, err))
	}
	defer csiConn.Close()

	req.VolumeContext = volumeCtx

	return ns.d.csiClientBuilder.NewNodeServiceClient(csiConn).StageVolume(ctx, req)
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if err := validateNodeUnstageVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Clear cache for this volume
	ns.nodeStageCacheMtx.Lock()
	delete(ns.nodeStageCache, req.GetVolumeId())
	ns.nodeStageCacheMtx.Unlock()

	csiConn, err := ns.d.csiClientBuilder.NewConnectionWithContext(ctx, ns.d.fwdEndpoint)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmtGrpcConnError(ns.d.fwdEndpoint, err))
	}
	defer csiConn.Close()

	return ns.d.csiClientBuilder.NewNodeServiceClient(csiConn).UnstageVolume(ctx, req)
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	nodeID, err := ns.metadata.GetInstanceID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to retrieve instance id of node: %v", err)
	}

	zone, err := ns.metadata.GetAvailabilityZone()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to retrieve availability zone of node: %v", err)
	}

	return &csi.NodeGetInfoResponse{
		NodeId: nodeID,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{topologyKey: zone},
		},
	}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.d.nscaps,
	}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	// Forward to the NFS CSI node plugin
	csiConn, err := ns.d.csiClientBuilder.NewConnectionWithContext(ctx, ns.d.fwdEndpoint)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmtGrpcConnError(ns.d.fwdEndpoint, err))
	}
	defer csiConn.Close()

	return ns.d.csiClientBuilder.NewNodeServiceClient(csiConn).GetVolumeStats(ctx, req)
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	// NFS expansion is controller-side only; no node-side expansion needed
	return nil, status.Error(codes.Unimplemented, "")
}
