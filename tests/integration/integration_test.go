// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/bianyuanop/oraclevm/tests" // include the tests that are shared between the integration and e2e

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/tests/integration"
	"github.com/bianyuanop/oraclevm/tests/workload"
	"github.com/bianyuanop/oraclevm/vm"

	lconsts "github.com/bianyuanop/oraclevm/consts"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "oraclevm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	testingNetworkConfig, err := workload.NewTestNetworkConfig(0)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	generator := workload.NewTxGenerator(testingNetworkConfig.Keys()[0])
	// Setup imports the integration test coverage
	integration.Setup(
		vm.New,
		testingNetworkConfig,
		lconsts.ID,
		generator,
		randomEd25519AuthFactory,
	)
})
