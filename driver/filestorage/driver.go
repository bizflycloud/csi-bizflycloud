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

package filestorage

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bizflycloud/csi-bizflycloud/driver/filestorage/csiclient"
	"github.com/bizflycloud/gobizfly"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/klog"
)

const (
	driverName    = "fs.csi.bizflycloud.vn"
	driverVersion = "0.1.0"
	topologyKey   = "topology." + driverName + "/zone"
)

var (
	serverGRPCEndpointCallCounter uint64
)

// DriverOpts contains configuration options for the file storage driver.
type DriverOpts struct {
	DriverName string
	ShareProto string
	ClusterID  string

	ServerCSIEndpoint string
	FwdCSIEndpoint    string

	CSIClientBuilder csiclient.Builder
}

// Driver implements the CSI file storage driver for Bizfly Cloud.
type Driver struct {
	name       string
	version    string
	shareProto string
	clusterID  string

	serverEndpoint string
	fwdEndpoint    string

	ids *identityServer
	cs  *controllerServer
	ns  *nodeServer

	vcaps  []*csi.VolumeCapability_AccessMode
	cscaps []*csi.ControllerServiceCapability
	nscaps []*csi.NodeServiceCapability

	csiClientBuilder csiclient.Builder
}

type nonBlockingGRPCServer struct {
	wg     sync.WaitGroup
	server *grpc.Server
}

// NewDriver creates a new file storage CSI driver.
func NewDriver(o *DriverOpts) (*Driver, error) {
	if o.ServerCSIEndpoint == "" {
		return nil, fmt.Errorf("server CSI endpoint is missing")
	}
	if o.FwdCSIEndpoint == "" {
		return nil, fmt.Errorf("forwarded CSI endpoint is missing")
	}

	name := o.DriverName
	if name == "" {
		name = driverName
	}

	shareProto := strings.ToUpper(o.ShareProto)
	if shareProto == "" {
		shareProto = "NFS"
	}

	d := &Driver{
		name:             name,
		version:          driverVersion,
		shareProto:       shareProto,
		clusterID:        o.ClusterID,
		csiClientBuilder: o.CSIClientBuilder,
	}

	serverProto, serverAddr, err := parseGRPCEndpoint(o.ServerCSIEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server endpoint address %s: %v", o.ServerCSIEndpoint, err)
	}

	fwdProto, fwdAddr, err := parseGRPCEndpoint(o.FwdCSIEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse forwarded endpoint address %s: %v", o.FwdCSIEndpoint, err)
	}

	d.serverEndpoint = endpointAddress(serverProto, serverAddr)
	d.fwdEndpoint = endpointAddress(fwdProto, fwdAddr)

	klog.Infof("Driver: %s", d.name)
	klog.Infof("Driver version: %s", d.version)
	klog.Infof("Share protocol: %s", d.shareProto)

	d.ids = &identityServer{d: d}

	return d, nil
}

// SetupControllerService configures the controller for share lifecycle management.
func (d *Driver) SetupControllerService(client *gobizfly.Client) error {
	klog.Info("Providing controller service")

	d.addControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	})

	d.addVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
	})

	d.cs = &controllerServer{d: d, client: client}
	return nil
}

// SetupNodeService configures the node service with forwarding to the NFS CSI driver.
func (d *Driver) SetupNodeService(metadata metadata.IMetadata) error {
	klog.Info("Providing node service")

	var supportsNodeStage bool

	nodeCapsMap, err := d.initProxiedDriver()
	if err != nil {
		return fmt.Errorf("failed to initialize proxied CSI driver: %v", err)
	}

	nscaps := make([]csi.NodeServiceCapability_RPC_Type, 0, len(nodeCapsMap))
	for c := range nodeCapsMap {
		nscaps = append(nscaps, c)
		if c == csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME {
			supportsNodeStage = true
		}
	}

	d.addNodeServiceCapabilities(nscaps)

	d.ns = &nodeServer{
		d:                 d,
		metadata:          metadata,
		supportsNodeStage: supportsNodeStage,
		nodeStageCache:    make(map[string]stageCacheEntry),
	}
	return nil
}

// Run starts the gRPC server.
func (d *Driver) Run() {
	if d.cs == nil && d.ns == nil {
		klog.Fatal("No CSI services initialized")
	}
	s := nonBlockingGRPCServer{}
	s.start(d.serverEndpoint, d.ids, d.cs, d.ns)
	s.wait()
}

func (d *Driver) addControllerServiceCapabilities(cs []csi.ControllerServiceCapability_RPC_Type) {
	caps := make([]*csi.ControllerServiceCapability, 0, len(cs))
	for _, c := range cs {
		klog.Infof("Enabling controller service capability: %v", c.String())
		caps = append(caps, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{Type: c},
			},
		})
	}
	d.cscaps = caps
}

func (d *Driver) addVolumeCapabilityAccessModes(vs []csi.VolumeCapability_AccessMode_Mode) {
	caps := make([]*csi.VolumeCapability_AccessMode, 0, len(vs))
	for _, c := range vs {
		klog.Infof("Enabling volume access mode: %v", c.String())
		caps = append(caps, &csi.VolumeCapability_AccessMode{Mode: c})
	}
	d.vcaps = caps
}

func (d *Driver) addNodeServiceCapabilities(ns []csi.NodeServiceCapability_RPC_Type) {
	caps := make([]*csi.NodeServiceCapability, 0, len(ns))
	for _, c := range ns {
		klog.Infof("Enabling node service capability: %v", c.String())
		caps = append(caps, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{Type: c},
			},
		})
	}
	d.nscaps = caps
}

// initProxiedDriver connects to the forwarded NFS CSI node plugin,
// probes it, and discovers its node capabilities.
func (d *Driver) initProxiedDriver() (csiNodeCapabilitySet, error) {
	conn, err := d.csiClientBuilder.NewConnection(d.fwdEndpoint)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s endpoint failed: %v", d.fwdEndpoint, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	identityClient := d.csiClientBuilder.NewIdentityServiceClient(conn)

	if err = identityClient.ProbeForever(ctx, conn, time.Second*5); err != nil {
		return nil, fmt.Errorf("probe failed: %v", err)
	}

	pluginInfo, err := identityClient.GetPluginInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin info of the proxied driver: %v", err)
	}

	klog.Infof("Proxying CSI driver %s version %s", pluginInfo.GetName(), pluginInfo.GetVendorVersion())

	nodeClient := d.csiClientBuilder.NewNodeServiceClient(conn)
	resp, err := nodeClient.GetCapabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node capabilities: %v", err)
	}

	caps := make(csiNodeCapabilitySet)
	for _, cap := range resp.GetCapabilities() {
		if cap.GetRpc() != nil {
			caps[cap.GetRpc().GetType()] = true
			klog.V(4).Infof("Discovered forwarded node capability: %v", cap.GetRpc().GetType())
		}
	}

	return caps, nil
}

func (s *nonBlockingGRPCServer) start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.wg.Add(1)
	go s.serve(endpoint, ids, cs, ns)
}

func (s *nonBlockingGRPCServer) wait() {
	s.wg.Wait()
}

func (s *nonBlockingGRPCServer) serve(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	defer s.wg.Done()

	proto, addr, err := parseGRPCEndpoint(endpoint)
	if err != nil {
		klog.Fatalf("couldn't parse GRPC server endpoint address %s: %v", endpoint, err)
	}

	if proto == "unix" {
		if err = os.Remove(addr); err != nil && !os.IsNotExist(err) {
			klog.Fatalf("failed to remove an existing socket file %s: %v", addr, err)
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		klog.Fatalf("listen failed for GRPC server: %v", err)
	}

	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		callID := atomic.AddUint64(&serverGRPCEndpointCallCounter, 1)
		klog.V(3).Infof("[ID:%d] GRPC call: %s", callID, info.FullMethod)
		klog.V(5).Infof("[ID:%d] GRPC request: %s", callID, protosanitizer.StripSecrets(req))
		resp, err := handler(ctx, req)
		if err != nil {
			klog.Errorf("[ID:%d] GRPC error: %v", callID, err)
		} else {
			klog.V(5).Infof("[ID:%d] GRPC response: %s", callID, protosanitizer.StripSecrets(resp))
		}
		return resp, err
	}))

	s.server = server

	if ids != nil {
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(server, ns)
	}

	klog.Infof("Listening for connections on %#v", listener.Addr())

	if err := server.Serve(listener); err != nil {
		klog.Fatalf("GRPC server failure: %v", err)
	}
}
