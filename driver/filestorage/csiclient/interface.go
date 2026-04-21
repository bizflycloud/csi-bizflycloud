// This file is part of csi-bizflycloud
//
// Copyright (C) 2020  Bizfly Cloud
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

package csiclient

import (
	"context"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// Node defines the interface for forwarding CSI Node RPCs to the NFS CSI driver.
type Node interface {
	GetCapabilities(ctx context.Context) (*csi.NodeGetCapabilitiesResponse, error)
	GetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error)

	StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error)
	UnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error)

	PublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error)
	UnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error)
}

// Identity defines the interface for forwarding CSI Identity RPCs.
type Identity interface {
	GetPluginInfo(ctx context.Context) (*csi.GetPluginInfoResponse, error)
	Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error)
	ProbeForever(ctx context.Context, conn *grpc.ClientConn, singleProbeTimeout time.Duration) error
}

// Builder defines the interface for creating gRPC connections and service clients
// to the forwarded NFS CSI node plugin.
type Builder interface {
	NewConnection(endpoint string) (*grpc.ClientConn, error)
	NewConnectionWithContext(ctx context.Context, endpoint string) (*grpc.ClientConn, error)

	NewNodeServiceClient(conn *grpc.ClientConn) Node
	NewIdentityServiceClient(conn *grpc.ClientConn) Identity
}
