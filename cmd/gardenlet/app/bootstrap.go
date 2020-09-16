// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// bootstrapKubeconfig retrieves an already existing kubeconfig for the Garden cluster from the Seed or bootstraps a new one
func bootstrapKubeconfig(
	ctx context.Context,
	logger *logrus.Logger,
	seedClient client.Client,
	config *config.GardenletConfiguration,
) (
	[]byte,
	string,
	string,
	error,
) {
	gardenKubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, seedClient, config.GardenClientConnection.KubeconfigSecret.Namespace, config.GardenClientConnection.KubeconfigSecret.Name)
	if err != nil {
		return nil, "", "", err
	}

	if len(gardenKubeconfig) > 0 {
		logger.Info("Found kubeconfig generated from bootstrap process. Using it")
		return gardenKubeconfig, "", "", nil
	}

	logger.Info("No kubeconfig from a previous bootstrap found. Starting bootstrap process.")

	if config.GardenClientConnection.BootstrapKubeconfig == nil {
		logger.Warn("Unable to perform kubeconfig bootstrapping. The gardenlet configuration `.gardenClientConnection.bootstrapKubeconfig` is not set")
		return nil, "", "", nil
	}

	bootstrapKubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, seedClient, config.GardenClientConnection.BootstrapKubeconfig.Namespace, config.GardenClientConnection.BootstrapKubeconfig.Name)
	if err != nil {
		return nil, "", "", err
	}

	if len(bootstrapKubeconfig) == 0 {
		logger.Warnf("unable to perform kubeconfig bootstrap. Bootstrap secret %s/%s does not contain a kubeconfig", config.GardenClientConnection.BootstrapKubeconfig.Namespace, config.GardenClientConnection.BootstrapKubeconfig.Name)
		return nil, "", "", nil
	}

	bootstrapClient, bootstrapRestConfig, err := bootstrap.CreateBootstrapClientFromKubeconfig(bootstrapKubeconfig)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to create bootstrap client from bootstrap kubeconfig: %v", err)
	}

	bootstrapTargetCluster := bootstraputil.GetTargetClusterName(config.SeedClientConnection)
	seedName := bootstraputil.GetSeedName(config.SeedConfig)

	logger.Info("Using provided bootstrap kubeconfig to request signed certificate.")

	return bootstrap.RequestBootstrapKubeconfig(ctx, logger, seedClient, bootstrapClient, bootstrapRestConfig, config.GardenClientConnection, seedName, bootstrapTargetCluster)
}
