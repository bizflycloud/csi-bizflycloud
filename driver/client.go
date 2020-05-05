package driver

import (
	"context"
	"errors"
	"github.com/bizflycloud/gobizfly"
)

// GetVolumesByName gets volumes by name of volume
func GetVolumesByName(ctx context.Context, client *gobizfly.Client, name string) (*gobizfly.Volume, error) {
	volumes, err := client.Volume.List(ctx, &gobizfly.ListOptions)

	for _, vol := range volumes {
		if vol.Name == name {
			return vol, nil
		}
	}
	return nil, errors.New("Volume not found")
}