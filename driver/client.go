package driver

import (
	"context"
	"errors"
	"fmt"
	"github.com/bizflycloud/gobizfly"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

const (
	volumeInUseStatus     = "in-use"

	diskAttachInitDelay = 1 * time.Second
	diskAttachFactor    = 1.2
	diskAttachSteps     = 15
	diskDetachInitDelay = 1 * time.Second
	diskDetachFactor    = 1.2
	diskDetachSteps     = 13
)

// GetVolumesByName gets volumes by name of volume
func GetVolumesByName(ctx context.Context, client *gobizfly.Client, name string) (*gobizfly.Volume, error) {
	volumes, err := client.Volume.List(ctx, &gobizfly.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, vol := range volumes {
		if vol.Name == name {
			return vol, nil
		}
	}
	return nil, errors.New("Volume not found")
}

// GetAttachmentDiskPath gets disk path in a server
func GetAttachmentDiskPath(ctx context.Context, client *gobizfly.Client, serverID string, volumeID string) (string, error) {
	volume, err := client.Volume.Get(ctx, volumeID)
	if err != nil {
		return "", err
	}
	if volume.Status != volumeInUseStatus {
		return "", fmt.Errorf("can not get device path of volume %s, its status is %s ", volume.Name, volume.Status)
	}
	volumeAttachments := volume.Attachments
	for _, att := range volumeAttachments {
		if att.ServerID == serverID {
			return att.Device, nil
		}
	}
	return "", errors.New("Attachment Disk Patch not found")
}

func WaitDiskAttached(ctx context.Context, client *gobizfly.Client, serverId string, volumeID string) error {
	backoff := wait.Backoff{
		Duration: diskAttachInitDelay,
		Factor:   diskAttachFactor,
		Steps:    diskAttachSteps,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		attached, err := diskIsAttached(ctx, client, serverId, volumeID)
		if err != nil {
			return false, err
		}
		return attached, nil
	})

	if err == wait.ErrWaitTimeout {
		err = fmt.Errorf("Volume %q failed to be attached within the alloted time", volumeID)
	}

	return err
}

// WaitDiskDetached waits for detached
func WaitDiskDetached(ctx context.Context, client *gobizfly.Client, serverId string, volumeID string) error {
	backoff := wait.Backoff{
		Duration: diskDetachInitDelay,
		Factor:   diskDetachFactor,
		Steps:    diskDetachSteps,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		attached, err := diskIsAttached(ctx, client, serverId, volumeID)
		if err != nil {
			return false, err
		}
		return !attached, nil
	})

	if err == wait.ErrWaitTimeout {
		err = fmt.Errorf("Volume %q failed to detach within the alloted time", volumeID)
	}

	return err
}

func diskIsAttached(ctx context.Context, client *gobizfly.Client, serverId string, volumeID string) (bool, error) {
	volume, err := client.Volume.Get(ctx, volumeID)
	if err != nil {
		return false, err
	}

	if len(volume.Attachments) > 0 {
		return serverId == volume.Attachments[0].ServerID, nil
	}

	return false, nil
}
