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

package csiclient

import (
	"context"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"google.golang.org/grpc"
)

var (
	_ Node     = &NodeSvcClient{}
	_ Identity = &IdentitySvcClient{}
)

// NodeSvcClient wraps the CSI NodeClient for forwarding RPCs to the NFS CSI driver.
type NodeSvcClient struct {
	cl csi.NodeClient
}

func (c *NodeSvcClient) GetCapabilities(ctx context.Context) (*csi.NodeGetCapabilitiesResponse, error) {
	return c.cl.NodeGetCapabilities(ctx, &csi.NodeGetCapabilitiesRequest{})
}

func (c *NodeSvcClient) GetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return c.cl.NodeGetVolumeStats(ctx, req)
}

func (c *NodeSvcClient) StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return c.cl.NodeStageVolume(ctx, req)
}

func (c *NodeSvcClient) UnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return c.cl.NodeUnstageVolume(ctx, req)
}

func (c *NodeSvcClient) PublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	return c.cl.NodePublishVolume(ctx, req)
}

func (c *NodeSvcClient) UnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return c.cl.NodeUnpublishVolume(ctx, req)
}

// IdentitySvcClient wraps the CSI IdentityClient for forwarding RPCs.
type IdentitySvcClient struct {
	cl csi.IdentityClient
}

func (c *IdentitySvcClient) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return c.cl.Probe(ctx, req)
}

func (c *IdentitySvcClient) ProbeForever(ctx context.Context, conn *grpc.ClientConn, singleProbeTimeout time.Duration) error {
	return rpc.ProbeForever(conn, singleProbeTimeout)
}

func (c *IdentitySvcClient) GetPluginInfo(ctx context.Context) (*csi.GetPluginInfoResponse, error) {
	return c.cl.GetPluginInfo(ctx, &csi.GetPluginInfoRequest{})
}
