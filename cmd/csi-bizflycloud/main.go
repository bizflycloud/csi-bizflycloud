package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	// "k8s.io/cloud-provider-openstack/pkg/csi/cinder"
	"github.com/bizflycloud/csi-bizflycloud/driver"
	"github.com/bizflycloud/gobizfly"
	"k8s.io/cloud-provider-openstack/pkg/csi/cinder/openstack"

	"k8s.io/cloud-provider-openstack/pkg/util/mount"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
)

var (
	endpoint string
	nodeID   string
	username string
	password string
	cluster  string
	api_url	 string
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {

	flag.CommandLine.Parse([]string{})

	cmd := &cobra.Command{
		Use:   "BizFlyCloudVolumeDriver",
		Short: "CSI based Cinder driver",
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
	cmd.MarkPersistentFlagRequired("nodeid")

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "CSI endpoint")
	cmd.MarkPersistentFlagRequired("endpoint")

	cmd.PersistentFlags().StringVar(&username, "username", "", "BizFly Cloud username")
	cmd.MarkPersistentFlagRequired("username")

	cmd.PersistentFlags().StringVar(&password, "password", "", "BizFly Cloud password")
	cmd.MarkPersistentFlagRequired("password")

	cmd.PersistentFlags().StringVar(&api_url, "api_url", "https://manage.bizflycloud.vn", "BizFly Cloud API URL")

	cmd.PersistentFlags().StringVar(&cluster, "cluster", "", "The identifier of the cluster that the plugin is running in.")

	// openstack.AddExtraFlags(pflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func handle() {

	d := driver.NewDriver(nodeID, endpoint, cluster)

	//Intiliaze mount
	mount, err := mount.GetMountProvider()
	if err != nil {
		klog.V(3).Infof("Failed to GetMountProvider: %v", err)
	}

	//Intiliaze Metadatda
	metadatda, err := openstack.GetMetadataProvider()
	if err != nil {
		klog.V(3).Infof("Failed to GetMetadataProvider: %v", err)
	}

	client, err := gobizfly.NewClient(gobizfly.WithTenantName(username), gobizfly.WithAPIUrl(api_url))

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	tok, err := client.Token.Create(ctx, &gobizfly.TokenCreateRequest{Username: username, Password: password})

	client.SetKeystoneToken(tok.KeystoneToken)

	if err != nil {
		klog.Warningf("Failed to GetOpenStackProvider: %v", err)
		return
	}

	d.SetupDriver(client, mount, metadatda)
	d.Run()
}
