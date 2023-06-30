// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	plugincmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"os"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/internal/version"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func CmdDel(args *skel.CmdArgs) (err error) {
	k8sArgs := plugincmd.K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		return fmt.Errorf("failed to load CNI ENV args: %w", err)
	}

	client, err := cmd.NewAgentOpenAPIUnixClient(constant.DefaultIPAMUnixSocketPath)
	if err != nil {
		return err
	}

	resp, err := client.Daemonset.GetCoordinatorConfig(daemonset.NewGetCoordinatorConfigParams().WithGetCoordinatorConfig(
		&models.GetCoordinatorArgs{
			PodName:      string(k8sArgs.K8S_POD_NAME),
			PodNamespace: string(k8sArgs.K8S_POD_NAMESPACE),
		},
	))
	if err != nil {
		return fmt.Errorf("failed to GetCoordinatorConfig: %v", err)
	}
	coordinatorConfig := resp.Payload

	conf, err := ParseConfig(args.StdinData, coordinatorConfig)
	if err != nil {
		return err
	}

	logger, err := logutils.SetupFileLogging(conf.LogOptions.LogLevel,
		conf.LogOptions.LogFilePath, conf.LogOptions.LogFileMaxSize,
		conf.LogOptions.LogFileMaxAge, conf.LogOptions.LogFileMaxCount)
	if err != nil {
		return fmt.Errorf("failed to init logger: %v ", err)
	}

	logger.Info("coordinator cmdDel starting", zap.String("Version", version.CoordinatorBuildDateVersion()), zap.String("Branch", version.CoordinatorGitBranch()),
		zap.String("Commit", version.CoordinatorGitCommit()),
		zap.String("Build time", version.CoordinatorBuildDate()),
		zap.String("Go Version", version.GoString()))

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "ADD"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)

	c := &coordinator{
		hostRuleTable: int(*conf.HostRuleTable),
	}

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			logger.Debug("Pod's netns already gone.  Nothing to do.")
			return nil
		}
		logger.Error("failed to GetNS", zap.Error(err))
		return err
	}
	defer c.netns.Close()

	if conf.TuneMode == ModeUnderlay {
		hostVeth := getHostVethName(args.ContainerID)
		vethLink, err := netlink.LinkByName(hostVeth)
		if err != nil {
			if _, ok := err.(*netlink.LinkNotFoundError); ok {
				logger.Debug("Host veth has gone, nothing to do", zap.String("HostVeth", hostVeth))
			} else {
				return fmt.Errorf("failed to get host veth device %s: %w", hostVeth, err)
			}
		}

		if err = netlink.LinkDel(vethLink); err != nil {
			logger.Error("failed to del hostVeth", zap.Error(err))
			return fmt.Errorf("failed to del hostVeth %s: %w", hostVeth, err)
		} else {
			logger.Error("success to del hostVeth", zap.String("HostVeth", hostVeth))
		}
	}

	err = c.netns.Do(func(netNS ns.NetNS) error {
		c.currentAddress, err = networking.GetAddersByName(args.IfName, netlink.FAMILY_ALL)
		if err != nil {
			logger.Error("failed to GetAddersByName", zap.String("interface", args.IfName))
			return err
		}
		return nil
	})

	if err != nil {
		// ignore err
		logger.Error("failed to GetAddersByName, ignore error")
		return nil
	}

	for idx := range c.currentAddress {
		ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
		err = networking.DelToRuleTable(ipNet, c.hostRuleTable)
		if err != nil && !os.IsNotExist(err) {
			logger.Error("failed to DelToRuleTable", zap.Int("HostRuleTable", c.hostRuleTable), zap.String("Dst", ipNet.String()), zap.Error(err))
			return fmt.Errorf("failed to DelToRuleTable: %v", err)
		}
	}
	logger.Info("coordinator cmdDel end")
	return nil
}