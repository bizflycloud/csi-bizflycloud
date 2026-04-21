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
	"errors"
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

const (
	bytesInGiB int64 = 1024 * 1024 * 1024

	shareCreating  = "creating"
	shareAvailable = "available"
	shareError     = "error"
	shareDeleting  = "deleting"
	shareExtending = "extending"

	shareDescription = "provisioned-by=fs.csi.bizflycloud.vn"
)

func bytesToGiB(sizeInBytes int64) int {
	sizeInGiB := int(sizeInBytes / bytesInGiB)
	if sizeInBytes%bytesInGiB > 0 {
		sizeInGiB++
	}
	return sizeInGiB
}

func parseGRPCEndpoint(endpoint string) (proto, addr string, err error) {
	const (
		unixScheme = "unix://"
		tcpScheme  = "tcp://"
	)

	if strings.HasPrefix(endpoint, "/") {
		return "unix", endpoint, nil
	}

	if strings.HasPrefix(endpoint, unixScheme) {
		pos := len(unixScheme)
		if endpoint[pos] != '/' {
			pos--
		}
		return "unix", endpoint[pos:], nil
	}

	if strings.HasPrefix(endpoint, tcpScheme) {
		return "tcp", endpoint[len(tcpScheme):], nil
	}

	return "", "", errors.New("endpoint uses unsupported scheme")
}

func endpointAddress(proto, addr string) string {
	return fmt.Sprintf("%s://%s", proto, addr)
}

func fmtGrpcConnError(fwdEndpoint string, err error) string {
	return fmt.Sprintf("connecting to fwd plugin at %s failed: %v", fwdEndpoint, err)
}

// parseExportLocation splits a Manila/Bizfly NFS export location
// (e.g., "10.0.0.5:/shares/share-xxx") into server and share path.
func parseExportLocation(exportLocation string) (server, share string, err error) {
	parts := strings.SplitN(exportLocation, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid export location format %q, expected server:/path", exportLocation)
	}
	return parts[0], parts[1], nil
}

//
// Request validation helpers
//

func validateCreateVolumeRequest(req *csi.CreateVolumeRequest) error {
	if req.GetName() == "" {
		return errors.New("volume name cannot be empty")
	}
	reqCaps := req.GetVolumeCapabilities()
	if reqCaps == nil {
		return errors.New("volume capabilities cannot be empty")
	}
	for _, cap := range reqCaps {
		if cap.GetBlock() != nil {
			return errors.New("block access type not allowed for file storage")
		}
	}
	return nil
}

func validateDeleteVolumeRequest(req *csi.DeleteVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID cannot be empty")
	}
	return nil
}

func validateControllerExpandVolumeRequest(req *csi.ControllerExpandVolumeRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID cannot be empty")
	}
	if req.GetCapacityRange() == nil {
		return errors.New("capacity range cannot be nil")
	}
	return nil
}

func validateValidateVolumeCapabilitiesRequest(req *csi.ValidateVolumeCapabilitiesRequest) error {
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return errors.New("volume capabilities cannot be nil or empty")
	}
	return nil
}

func validateNodeStageVolumeRequest(req *csi.NodeStageVolumeRequest) error {
	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}
	if req.GetVolumeContext() == nil || len(req.GetVolumeContext()) == 0 {
		return errors.New("volume context cannot be nil or empty")
	}
	return nil
}

func validateNodeUnstageVolumeRequest(req *csi.NodeUnstageVolumeRequest) error {
	if req.GetStagingTargetPath() == "" {
		return errors.New("staging path missing in request")
	}
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}
	return nil
}

func validateNodePublishVolumeRequest(req *csi.NodePublishVolumeRequest) error {
	if req.GetVolumeCapability() == nil {
		return errors.New("volume capability missing in request")
	}
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}
	if req.GetTargetPath() == "" {
		return errors.New("target path missing in request")
	}
	return nil
}

func validateNodeUnpublishVolumeRequest(req *csi.NodeUnpublishVolumeRequest) error {
	if req.GetTargetPath() == "" {
		return errors.New("target path missing in request")
	}
	if req.GetVolumeId() == "" {
		return errors.New("volume ID missing in request")
	}
	return nil
}

// csiNodeCapabilitySet is a set of CSI Node capabilities keyed by capability type.
type csiNodeCapabilitySet map[csi.NodeServiceCapability_RPC_Type]bool


