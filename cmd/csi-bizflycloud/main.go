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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bizflycloud/csi-bizflycloud/driver"
	"github.com/bizflycloud/gobizfly"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/cloud-provider-openstack/pkg/util/mount"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
)

var (
	endpoint       string
	nodeID         string
	authMethod     string
	username       string
	password       string
	tenantID       string
	appCredID      string
	appCredSecret  string
	cluster        string
	apiUrl         string
	region         string
	isControlPlane bool
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {

	flag.CommandLine.Parse([]string{})

	cmd := &cobra.Command{
		Use:   "BizFlyCloudVolumeDriver",
		Short: "CSI based BizFly Cloud Volume driver",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Glog requires this otherwise it complains.
			flag.CommandLine.Parse(nil)

			// This is a temporary hack to enable proper logging until upstream dependencies
			// are migrated to fully utilize klog instead of glog.
			klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
			klog.InitFlags(klogFlags)

			// Sync the glog and klog flags.
			cmd.Flags().VisitAll(func(f1 *pflag.Flag) {
				f2 := klogFlags.Lookup(f1.Name)
				if f2 != nil {
					value := f1.Value.String()
					f2.Value.Set(value)
				}
			})
		},
		Run: func(cmd *cobra.Command, args []string) {
			handle()
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&nodeID, "nodeid", "", "node id")
	cmd.PersistentFlags().MarkDeprecated("nodeid", "This flag would be removed in future. Currently, the value is ignored by the driver")

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "CSI endpoint")
	cmd.MarkPersistentFlagRequired("endpoint")

	cmd.PersistentFlags().BoolVar(&isControlPlane, "is_control_plane", false, "Is Control Plane node")

	cmd.PersistentFlags().StringVar(&authMethod, "auth_method", "password", "Authentication method")

	cmd.PersistentFlags().StringVar(&username, "username", "", "BizFly Cloud username")

	cmd.PersistentFlags().StringVar(&password, "password", "", "BizFly Cloud password")

	cmd.PersistentFlags().StringVar(&appCredID, "application_credential_id", "", "BizFly Cloud Application Credential ID")

	cmd.PersistentFlags().StringVar(&appCredSecret, "application_credential_secret", "", "BizFly Cloud Application Credential Secret")

	cmd.PersistentFlags().StringVar(&tenantID, "tenant_id", "", "BizFly Cloud Tenant ID")

	cmd.PersistentFlags().StringVar(&apiUrl, "api_url", "https://manage.bizflycloud.vn", "BizFly Cloud API URL")

	cmd.PersistentFlags().StringVar(&cluster, "cluster", "", "The identifier of the cluster that the plugin is running in.")

	cmd.PersistentFlags().StringVar(&region, "region", "HN", "BizFly Cloud Region Name. Available: HN and HCM")

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func handle() {

	d := driver.NewDriver(endpoint, cluster)

	// Intiliaze mount
	iMount := mount.GetMountProvider()
	//Intiliaze Metadatda
	metadataProvider := metadata.GetMetadataProvider("metadataService")
	if isControlPlane {
		client, err := gobizfly.NewClient(gobizfly.WithAPIUrl(apiUrl), gobizfly.WithProjectId(tenantID), gobizfly.WithRegionName(region))
		if err != nil {
			klog.Errorf("failed to create bizfly client: %v", err)
			return
		}
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
		defer cancelFunc()

		tok, err := client.Token.Create(ctx, &gobizfly.TokenCreateRequest{
			AuthMethod:    authMethod,
			Username:      username,
			Password:      password,
			AppCredID:     appCredID,
			AppCredSecret: appCredSecret})

		if err != nil {
			klog.Errorf("Failed to get bizfly client token: %v", err)
			return
		}

		client.SetKeystoneToken(tok)
		d.SetupControlDriver(client, iMount, metadataProvider)
		d.Run()
	} else {
		d.SetupNodeDriver(iMount, metadataProvider)
		d.Run()
	}

}
