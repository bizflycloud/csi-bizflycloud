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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bizflycloud/csi-bizflycloud/driver/filestorage"
	"github.com/bizflycloud/csi-bizflycloud/driver/filestorage/csiclient"
	"github.com/bizflycloud/gobizfly"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-openstack/pkg/util/metadata"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
)

var (
	endpoint      string
	fwdEndpoint   string
	driverName    string
	protoSelector string
	clusterID     string

	authMethod    string
	username      string
	password      string
	tenantID      string
	appCredID     string
	appCredSecret string
	apiUrl        string
	region        string

	provideControllerService bool
	provideNodeService       bool
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	flag.CommandLine.Parse([]string{})

	cmd := &cobra.Command{
		Use:   "csi-fs-bizflycloud",
		Short: "CSI File Storage driver for Bizfly Cloud",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			flag.CommandLine.Parse(nil)

			klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
			klog.InitFlags(klogFlags)

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

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	cmd.PersistentFlags().StringVar(&fwdEndpoint, "fwdendpoint", "", "CSI Node Plugin endpoint to forward Node Service RPCs to (e.g., NFS CSI driver)")
	cmd.MarkPersistentFlagRequired("fwdendpoint")

	cmd.PersistentFlags().StringVar(&driverName, "drivername", "fs.csi.bizflycloud.vn", "Name of the driver")
	cmd.PersistentFlags().StringVar(&protoSelector, "share-protocol-selector", "NFS", "Share protocol to use (default: NFS)")
	cmd.PersistentFlags().StringVar(&clusterID, "cluster", "", "The identifier of the cluster")

	cmd.PersistentFlags().StringVar(&authMethod, "auth_method", "password", "Authentication method")
	cmd.PersistentFlags().StringVar(&username, "username", "", "Bizfly Cloud username")
	cmd.PersistentFlags().StringVar(&password, "password", "", "Bizfly Cloud password")
	cmd.PersistentFlags().StringVar(&appCredID, "application_credential_id", "", "Bizfly Cloud Application Credential ID")
	cmd.PersistentFlags().StringVar(&appCredSecret, "application_credential_secret", "", "Bizfly Cloud Application Credential Secret")
	cmd.PersistentFlags().StringVar(&tenantID, "tenant_id", "", "Bizfly Cloud Tenant ID")
	cmd.PersistentFlags().StringVar(&apiUrl, "api_url", "https://manage.bizflycloud.vn", "Bizfly Cloud API URL")
	cmd.PersistentFlags().StringVar(&region, "region", "HaNoi", "Bizfly Cloud Region Name (HaNoi, HoChiMinh)")

	cmd.PersistentFlags().BoolVar(&provideControllerService, "provide-controller-service", true, "Provide the controller service (default: true)")
	cmd.PersistentFlags().BoolVar(&provideNodeService, "provide-node-service", true, "Provide the node service (default: true)")

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func handle() {
	csiClientBuilder := &csiclient.ClientBuilder{}

	opts := &filestorage.DriverOpts{
		DriverName:        driverName,
		ShareProto:        protoSelector,
		ClusterID:         clusterID,
		ServerCSIEndpoint: endpoint,
		FwdCSIEndpoint:    fwdEndpoint,
		CSIClientBuilder:  csiClientBuilder,
	}

	d, err := filestorage.NewDriver(opts)
	if err != nil {
		klog.Fatalf("Driver initialization failed: %v", err)
	}

	if provideControllerService {
		client, err := gobizfly.NewClient(
			gobizfly.WithAPIURL(apiUrl),
			gobizfly.WithProjectID(tenantID),
			gobizfly.WithRegionName(region),
		)
		if err != nil {
			klog.Fatalf("Failed to create Bizfly Cloud client: %v", err)
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
		defer cancelFunc()

		tok, err := client.Token.Init(ctx, &gobizfly.TokenCreateRequest{
			AuthMethod:    authMethod,
			Username:      username,
			Password:      password,
			AppCredID:     appCredID,
			AppCredSecret: appCredSecret,
		})
		if err != nil {
			klog.Fatalf("Failed to authenticate with Bizfly Cloud: %v", err)
		}

		client.SetKeystoneToken(tok)

		if err := d.SetupControllerService(client); err != nil {
			klog.Fatalf("Controller service initialization failed: %v", err)
		}
	}

	if provideNodeService {
		metadataProvider := metadata.GetMetadataProvider("metadataService")

		if err := d.SetupNodeService(metadataProvider); err != nil {
			klog.Fatalf("Node service initialization failed: %v", err)
		}
	}

	d.Run()
}
