package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/cert-manager/cert-manager/test/acme/dns"
	"k8s.io/klog/v2"
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
	fixture := dns.NewFixture(&heProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/he"),
		dns.SetDNSServer("ns1.he.net:53"),
		dns.SetStrict(true),
		dns.SetPropagationLimit(1800*time.Second),
		dns.SetResolvedFQDN("_acme-challenge-test."+zone),
	)

	fixture.RunConformance(t)
}
