package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/klog/v2"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"

	"github.com/waldner/cert-manager-webhook-he/utils"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {

	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&heProviderSolver{},
	)
}

// heProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type heProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client *kubernetes.Clientset
}

type secretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// heProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
type heProviderConfig struct {
	// Depending on the method used, some of these fields will be empty/unused
	CredentialsSecretRef secretRef `json:"credentialsSecretRef"`
	ApiKeySecretRef      secretRef `json:"ApiKeySecretRef"`
	HeUrl                string    `json:"heUrl"`
	Method               string    `json:"method"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *heProviderSolver) Name() string {
	return "he"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *heProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {

	hc, err := c.initConfig(ch)
	if err != nil {
		return err
	}

	if hc.Method == "login" {
		err = hc.AddTxtRecordWithLogin(ch)
	} else {
		err = hc.AddTxtRecordWithDynamicDns(ch)
	}

	if err != nil {
		klog.ErrorS(err, "Error during Present")
	}
	return err
}

// CleanUp  should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *heProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {

	hc, err := c.initConfig(ch)
	if err != nil {
		return err
	}

	if hc.Method == "login" {
		err = hc.RemoveTxtRecordWithLogin(ch)
	} else {
		err = hc.RemoveTxtRecordWithDynamicDns(ch)
	}

	if err != nil {
		klog.ErrorS(err, "Error during CleanUp")
	}
	return err
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *heProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	///// UNCOMMENT THE BELOW CODE TO MAKE A KUBERNETES CLIENTSET AVAILABLE TO
	///// YOUR CUSTOM DNS PROVIDER

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("error running NewForConfig: %+v", err)
	}

	c.client = cl
	///// END OF CODE TO MAKE KUBERNETES CLIENTSET AVAILABLE
	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (heProviderConfig, error) {
	cfg := heProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *heProviderSolver) initConfig(ch *v1alpha1.ChallengeRequest) (*utils.HeClient, error) {

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	if cfg.Method != "" && cfg.Method != "login" && cfg.Method != "dynamic-dns" {
		return nil, fmt.Errorf("invalid configuration method '%v', valid values are 'login' or 'dynamic-dns'", cfg.Method)
	}

	// fill in config defaults
	if cfg.Method == "" {
		cfg.Method = "login"
	}

	if cfg.Method == "login" {
		if cfg.HeUrl == "" {
			cfg.HeUrl = "https://dns.he.net/"
		}
	} else {
		if cfg.HeUrl == "" {
			cfg.HeUrl = "https://dyn.dns.he.net/"
		}
	}
	// add trailing slash to heUrl if not present
	if cfg.HeUrl[len(cfg.HeUrl)-1:] != "/" {
		cfg.HeUrl += "/"
	}

	heClient := &utils.HeClient{
		Method: cfg.Method,
		HeUrl:  cfg.HeUrl,
	}

	useSecrets := os.Getenv("USE_SECRETS")

	if useSecrets == "true" {
		err = c.populateClientFromSecrets(heClient, cfg, ch)
	} else {
		err = c.populateClientFromEnv(heClient, cfg, ch)
	}

	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: %v", err)
	}

	client := &http.Client{
		Jar: jar,
	}
	heClient.Client = client

	klog.V(4).InfoS("Generated config", "heClient", heClient)

	return heClient, nil
}

func (c *heProviderSolver) populateClientFromSecrets(heClient *utils.HeClient, cfg heProviderConfig, ch *v1alpha1.ChallengeRequest) error {

	secretData := map[string]*map[string][]byte{}
	secretNamespaces := map[string]string{}

	if cfg.Method == "login" {
		if cfg.CredentialsSecretRef.Name == "" {
			cfg.CredentialsSecretRef.Name = "he-credentials"
		}
		secretData[cfg.CredentialsSecretRef.Name] = &map[string][]byte{}
		if cfg.CredentialsSecretRef.Namespace != "" {
			secretNamespaces[cfg.CredentialsSecretRef.Name] = cfg.CredentialsSecretRef.Namespace
		} else {
			secretNamespaces[cfg.CredentialsSecretRef.Name] = ch.ResourceNamespace
		}
	} else {
		if cfg.ApiKeySecretRef.Name == "" {
			cfg.ApiKeySecretRef.Name = "he-credentials"
		}
		secretData[cfg.ApiKeySecretRef.Name] = &map[string][]byte{}
		if cfg.ApiKeySecretRef.Namespace != "" {
			secretNamespaces[cfg.ApiKeySecretRef.Name] = cfg.ApiKeySecretRef.Namespace
		} else {
			secretNamespaces[cfg.ApiKeySecretRef.Name] = ch.ResourceNamespace
		}
	}

	// read the secret(s)
	for secretName := range secretData {

		sec, err := c.client.CoreV1().Secrets(secretNamespaces[secretName]).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("unable to read secret `%s/%s`: %v", secretNamespaces[secretName], secretName, err)
		}
		secretData[secretName] = &sec.Data
	}

	if cfg.Method == "login" {
		usernameKey := "username"
		passwordKey := "password"
		username, err := getKeyFromSecret(secretData[cfg.CredentialsSecretRef.Name], usernameKey)
		if err != nil {
			return fmt.Errorf("unable to get %v from secret `%s/%s`; %v", usernameKey, ch.ResourceNamespace, cfg.CredentialsSecretRef.Name, err)
		}
		heClient.Username = username

		password, err := getKeyFromSecret(secretData[cfg.CredentialsSecretRef.Name], passwordKey)
		if err != nil {
			return fmt.Errorf("unable to get %v from secret `%s/%s`; %v", passwordKey, ch.ResourceNamespace, cfg.CredentialsSecretRef.Name, err)
		}
		heClient.Password = password
	} else {
		// extract credentials
		apiKeyKey := "apiKey"

		apiKey, err := getKeyFromSecret(secretData[cfg.ApiKeySecretRef.Name], apiKeyKey)
		if err != nil {
			return fmt.Errorf("unable to get %v from secret `%s/%s`; %v", apiKeyKey, ch.ResourceNamespace, cfg.ApiKeySecretRef.Name, err)
		}
		heClient.ApiKey = apiKey
	}
	return nil
}

func (c *heProviderSolver) populateClientFromEnv(heClient *utils.HeClient, cfg heProviderConfig, ch *v1alpha1.ChallengeRequest) error {
	if cfg.Method == "login" {
		heClient.Username = os.Getenv("HE_USERNAME")
		heClient.Password = os.Getenv("HE_PASSWORD")
	} else {
		heClient.ApiKey = os.Getenv("HE_APIKEY")
	}
	return nil
}

// extract a key from a secret
func getKeyFromSecret(secretData *map[string][]byte, key string) (string, error) {

	data, ok := (*secretData)[key]

	if !ok {
		return "", fmt.Errorf("key %q not found in secret data", key)
	}
	d := string(data)
	if d == "" {
		return "", fmt.Errorf("value for key %q is empty", key)
	}
	return d, nil
}
