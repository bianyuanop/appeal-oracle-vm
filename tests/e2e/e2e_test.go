// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	_ "github.com/bianyuanop/oraclevm/tests" // include the tests that are shared between the integration and e2e

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/bianyuanop/oraclevm/consts"
	"github.com/bianyuanop/oraclevm/tests/workload"
	"github.com/bianyuanop/oraclevm/throughput"
	"github.com/bianyuanop/oraclevm/vm"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "oraclevm-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "oraclevm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single oraclevm Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	testingNetworkConfig, err := workload.NewTestNetworkConfig(100 * time.Millisecond)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject oraclevm name
	// and workload factory to orchestrate the test.
	spamHelper := throughput.SpamHelper{
		KeyType: auth.ED25519Key,
	}

	firstKey := testingNetworkConfig.Keys()[0]
	generator := workload.NewTxGenerator(firstKey)
	spamKey := &auth.PrivateKey{
		Address: auth.NewED25519Address(firstKey.PublicKey()),
		Bytes:   firstKey[:],
	}
	tc := e2e.NewTestContext()
	he2e.SetWorkload(testingNetworkConfig, generator, expectedABI, &spamHelper, spamKey)

	return fixture.NewTestEnvironment(tc, flagVars, owner, testingNetworkConfig, consts.ID).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})
