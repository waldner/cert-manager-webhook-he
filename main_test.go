package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"k8s.io/klog/v2"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuiteLogin(t *testing.T) {

	verbose := os.Getenv("VERBOSE")
	if verbose != "" {
		klog.InitFlags(nil)
		flag.Set("v", "5")
		flag.Parse()
		defer klog.Flush()
	}

	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	// Uncomment the below fixture when implementing your custom DNS provider
	fixture := acmetest.NewFixture(&heProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath("testdata/he"),
		acmetest.SetDNSServer("ns1.he.net:53"),
		acmetest.SetStrict(true),
		acmetest.SetPropagationLimit(1800*time.Second),
		acmetest.SetResolvedFQDN("_acme-challenge-test."+zone),
	)

	//need to uncomment and  RunConformance delete runBasic and runExtended once https://github.com/cert-manager/cert-manager/pull/4835 is merged
	//fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)

}
