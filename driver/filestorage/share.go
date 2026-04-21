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
	"errors"
	"fmt"
	"time"

	"github.com/bizflycloud/gobizfly"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

const (
	waitForAvailableShareTimeout = 3
	waitForAvailableShareRetries = 10
)

// getOrCreateShare retrieves an existing share by name, or creates a new one
// if it doesn't exist yet. Waits until the share is in "available" status.
func getOrCreateShare(ctx context.Context, client *gobizfly.Client, shareName string, createReq *gobizfly.CreateShareRequest) (*gobizfly.Share, error) {
	// First, check if the share already exists
	share, err := getShareByName(ctx, client, shareName)
	if err == nil && share != nil {
		klog.V(4).Infof("volume %s already exists with ID %s", shareName, share.ID)
		if share.Status == shareAvailable {
			return share, nil
		}
		return waitForShareAvailable(ctx, client, share.ID)
	}

	// Share doesn't exist, create it
	share, err = client.FileStorage.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create share %s: %v", shareName, err)
	}

	klog.V(4).Infof("created share %s with ID %s, waiting for available status", shareName, share.ID)
	return waitForShareAvailable(ctx, client, share.ID)
}

// getShareByName lists all shares and returns the one matching the given name.
func getShareByName(ctx context.Context, client *gobizfly.Client, name string) (*gobizfly.Share, error) {
	shares, err := client.FileStorage.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range shares {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("share with name %s not found", name)
}

// getShareByID lists all shares and returns the one matching the given ID.
func getShareByID(ctx context.Context, client *gobizfly.Client, id string) (*gobizfly.Share, error) {
	shares, err := client.FileStorage.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range shares {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, gobizfly.ErrNotFound
}

// waitForShareAvailable polls the share status until it becomes "available".
func waitForShareAvailable(ctx context.Context, client *gobizfly.Client, shareID string) (*gobizfly.Share, error) {
	var share *gobizfly.Share

	backoff := wait.Backoff{
		Duration: time.Second * waitForAvailableShareTimeout,
		Factor:   1.2,
		Steps:    waitForAvailableShareRetries,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		shares, listErr := client.FileStorage.List(ctx)
		if listErr != nil {
			return false, listErr
		}
		
		found := false
		for _, s := range shares {
			if s.ID == shareID {
				share = s
				found = true
				break
			}
		}

		if !found {
			return false, fmt.Errorf("share %s not found in list", shareID)
		}

		switch share.Status {
		case shareAvailable:
			return true, nil
		case shareCreating, shareExtending:
			klog.V(4).Infof("share %s is in %s state, waiting...", shareID, share.Status)
			return false, nil
		case shareError:
			return false, fmt.Errorf("share %s is in error state", shareID)
		default:
			return false, fmt.Errorf("share %s is in unexpected state: %s", shareID, share.Status)
		}
	})

	if err != nil {
		return share, err
	}
	return share, nil
}

// deleteShare deletes a share by ID. Returns nil if the share is already deleted (not found).
func deleteShare(ctx context.Context, client *gobizfly.Client, shareID string) error {
	err := client.FileStorage.Delete(ctx, shareID, false)
	if err != nil {
		if errors.Is(err, gobizfly.ErrNotFound) {
			klog.V(4).Infof("share %s not found, assuming already deleted", shareID)
			return nil
		}
		return err
	}
	return nil
}

// resizeShare resizes a share to the desired size in GiB and waits for it to become available.
func resizeShare(ctx context.Context, client *gobizfly.Client, shareID string, newSizeGiB int) (*gobizfly.Share, error) {
	share, err := client.FileStorage.Resize(ctx, shareID, &gobizfly.ResizeShareRequest{
		NewSize: newSizeGiB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resize share %s: %v", shareID, err)
	}
	return waitForShareAvailable(ctx, client, share.ID)
}
