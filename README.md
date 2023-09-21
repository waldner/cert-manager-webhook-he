# `cert-Manager` ACME DNS01 Webhook Solver for HE DNS

[![Go Report Card](https://goreportcard.com/badge/github.com/waldner/cert-manager-webhook-he)](https://goreportcard.com/report/github.com/waldner/cert-manager-webhook-he)
[![Releases](https://img.shields.io/github/v/tag/waldner/cert-manager-webhook-he)](https://github.com/waldner/cert-manager-webhook-he/tags)
[![LICENSE](https://img.shields.io/github/license/waldner/cert-manager-webhook-he)](https://github.com/waldner/cert-manager-webhook-he/blob/master/LICENSE)

A webhook to use [HE DNS](https://dns.he.net) as a DNS01 ACME Issuer for [cert-manager](https://github.com/jetstack/cert-manager).


## Installation

The webhook must be installed in the same namespace where cert-manager is running (usually `cert-manager`).
See the following paragraph for an explanation of what "use secrets" or "use environment variables" mean.

To install with helm from the registry, run:

```bash
# to use secrets for credentials:
$ helm upgrade --install --namespace cert-manager --set auth.useSecrets=true cert-manager-webhook-he oci://ghcr.io/waldner/charts/cert-manager-webhook-he

# to use environment variables
$ helm upgrade --install --namespace cert-manager \
   --set auth.heUsername=myusername \
   --set auth.hePassword=mypassword \
   --set auth.heApiKey=myapikey \
   cert-manager-webhook-he oci://ghcr.io/waldner/charts/cert-manager-webhook-he
```

If you want to install from the repo checkout:

```bash
$ git clone https://github.com/waldner/cert-manager-webhook-he.git
$ cd cert-manager-webhook-he

# to use secrets for credentials:
$ helm upgrade --install --namespace cert-manager \
     --set auth.useSecret=true cert-manager-webhook-he deploy/cert-manager-webhook-he

# to use environment variables:
$ helm upgrade --install --namespace cert-manager \
   --set auth.heUsername=myusername \
   --set auth.hePassword=mypassword \
   --set auth.heApiKey=myapikey \
   cert-manager-webhook-he deploy/cert-manager-webhook-he
```

Check the logs with

```bash
$ kubectl get pods -n cert-manager --watch
$ kubectl logs -n cert-manager cert-manager-webhook-he-xxxxx
```


## Concepts and configuration

The webhook can work in two modes: `login` and `dynamic-dns` (explained later).
Whatever method you choose, in the `Issuer` YAML the configuration options must
be under the `dns01.webhook` path (see examples below).
Also regardless of the mode, the webhook can read its credentials either from
environment variables (the default) or from kubernetes `Secret`s.

The main difference is that credentials passed via environment variables are static
and can only be changed by redeploying the container, while credentials stored
in secrets can be updated by just updating the secrets (or creating now ones),
then referencing them from the `Issuer`.

If you want to use multiple accounts, or be able to set per-issuer credentials,
you should use secrets. If, on the other hand, you only have a single set of
credentials that you want to use everywhere, using environment variables is
appropriate.

Whether the webhook reads the credentials from environment variables (the
default) or from secrets is determined by the `auth.useSecrets` variable of the
Helm chart, which you can override when you deploy the chart.

Choosing to use secrets or environment variables has implications for the
deployment, since when using secrets additional permissions will be given to
the webhook service account to be able to read secrets (see below for details).

### `login` mode

In `login` mode, the TXT record(s) are created and deleted by logging into the
HE DNS control panel using the normal user credentials. The credentials needed
for this mode are the HE DNS control panel username and password. If you store
them in a secret, they must be associated respectively to the `username` and 
`password` keys in the secret data. Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: he-credentials
type: Opaque
stringData:
  username: "myHEusername"
  password: "myHEpassword"
```

If you use environment variables, you must pass them as `auth.heUsername` and
`auth.hePassword` when deploying the Helm chart.

Here's a sample `Issuer` configuration for the `login` mode:

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
...
  solvers:
    - dns01:
      webhook:
        solverName: he
        groupName: acme.xdb.me
        config:
          heUrl: "https://dns.he.net"   # URL for operations. Default (and probably the only valid value): "https://dns.he.net"
          method: "login"               # method to use. "login" is also the default
          # only if you use secrets
          credentialsSecretRef:
            name: "my-secret"           # name of secret. Default: "he-credentials"
            namespace: "myns"           # optional namespace for the secret. If not given, the secret is
                                        # looked for in the issuer namespace.
                                        # For a ClusterIssuer, specify this or the release namespace (eg,
                                        # `cert-manager`) will be used.
```


### `dynamic-dns` mode

In `dynamic-dns` mode, the TXT record(s) are never created or deleted, but 
instead you need to pre-create an aptly named TXT entry (eg, 
`_acme-challenge.mydomain.com`) in the domain control panel and the webhook
will update/overwrite it with the actual key needed to solve the ACME
challenge. To do this, you also need to generate or set an API key (done via
the control panel for the record) that will be used for the dynamic update
requests.

*NOTE: The `dynamic-dns` mode cannot do concurrent validations (it's always
the same TXT record that gets updated), so it should only be used in environments
where you expect only a single challenge at a time for each domain, and you know
in advance the name of the TXT record to update.*

For more information, see the section "Dynamic TXT records" [here](https://dns.he.net/).

For this mode, the only credential you need is the API key. If you want to
use a secret, you store it in the `apiKey` field. Here's an example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: he-credentials
type: Opaque
stringData:
  apiKey: "skdhjfkdhkjs"
```

If you pass it via the environment, you must use the `auth.heApiKey`
variable when deploying the Helm chart.

Here's a sample `Issuer` configuration for the `dynamic-dns` mode:

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
...
  solvers:
    - dns01:
      webhook:
        solverName: he
        groupName: acme.xdb.me
        config:
          heUrl: "https://dyn.dns.he.net" # URL for operations. Default: "https://dyn.dns.he.net"
          method: "dynamic-dns"           # method to use.

          # Only if you use secrets
          apiKeySecretRef:
            name: "my-secret"             # name of secret. Default: "he-credentials"
            namespace: "myns"             # optional namespace for the secret. If not given, the secret is
                                          # looked for in the issuer namespace.
                                          # For a ClusterIssuer, specify this or the release namespace (eg,
                                          # `cert-manager`) will be used.
```

### Access control for secrets

If using secrets, there is the option to limit the namespaces the webhook will
be able to access, and also the name of the secrets it will have permission to
read. This is done by setting the helm variables `rbac.secretNamespaces` (a list
of namespaces, by default `[default]`) and `rbac.secretNames` (a list of names,
by default `[he-credentials]`).
If you want to be able to read secrets in any namespace, pass an empty list for
`rbac.secretNamespaces`, and a `ClusterRole` will be created instead of a `Role`
(use with caution).



## Development

*IMPORTANT NOTE: only the `login` mode is conformant with the cert-manager
requirements, as it allows for multiple simultaneous DNS01 challenges (and
thus TXT records) in a single domain. The `dynamic-dns` mode cannot do that
(it's always the same TXT record that gets updated), so it should only be
used in environments where you expect only a single challenge at a time for
each domain.*

For the same reason, a `dynamic-dns` mode test is not included in the test
suite, as it expects the TXT record to be removed for the test to be declared
successful (`dynamic-dns` mode merely overwrites the key with new values every
time; the TXT record is never removed).

Fortunately, in the actual runtime cert-manager doesn't check that a given
record is deleted or not after the challenge, so that's why you can use the
`dynamic-dns` method if you want (but not run tests with it), modulo the above
notice.


### Running the test suite

Conformance testing is achieved through Kubernetes emulation via the
kubebuilder-tools suite, in conjunction with real calls to HE on a
test domain, using valid credentials or API token stored in secrets.

The test configures a `_acme-challenge-test` TXT entry, attempts to verify
its presence, and removes the entry, thereby verifying the Prepare and CleanUp
functions.

To run the test suite, you must create two files under `testdata/he`. One 
(let's call it `config.json`) represents the configuration fragment that
will be used by the webhook; the other one (`secret.yaml`) must contain the
HE credentials (username and password). There are examples of both files
under `testdata/he`.

Once the files are in place, run the test suite with:

```bash
TEST_ZONE_NAME=yourdomain.com. make test
```

You can also set `VERBOSE=1` (or to any other nonempty value) to see debug messages
(note that this increases verbosity for all components):

```bash
VERBOSE=1 TEST_ZONE_NAME=yourdomain.com. make test
```

Have a look at `main_test.go` in case you want to customize the test suite.
