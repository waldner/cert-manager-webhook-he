OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

# not used
IMAGE_NAME := "mmoerz/cert-manager-webhook-he"
IMAGE_TAG := "0.0.5"

OUT := $(shell pwd)/_out

KUBE_VERSION=1.30.0

USE_SECRETS ?= false
HE_USERNAME ?= ""
HE_PASSWORD ?= ""
HE_APIKEY ?= "" 

$(shell mkdir -p "$(OUT)")
export TEST_ASSET_ETCD=_test/kubebuilder/bin/etcd
export TEST_ASSET_KUBE_APISERVER=_test/kubebuilder/bin/kube-apiserver
export TEST_ASSET_KUBECTL=_test/kubebuilder/bin/kubectl

test: _test/kubebuilder
	USE_SECRETS=true go test -v .

_test/kubebuilder:
	curl -fsSL https://go.kubebuilder.io/test-tools/$(KUBE_VERSION)/$(OS)/$(ARCH) -o kubebuilder-tools.tar.gz
	mkdir -p _test/kubebuilder
	tar -xvf kubebuilder-tools.tar.gz
	mv kubebuilder/bin _test/kubebuilder/
	rm kubebuilder-tools.tar.gz
	rm -R kubebuilder

clean: clean-kubebuilder

clean-kubebuilder:
	rm -Rf _test/kubebuilder

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
      --set image.repository=$(IMAGE_NAME) \
      --set image.tag=$(IMAGE_TAG) \
	  --set auth.useSecrets=$(USE_SECRETS) \
	  --set auth.heUsername=$(HE_USERNAME) \
 	  --set auth.hePassword=$(HE_PASSWORD) \
  	  --set auth.heApiKey=$(HE_APIKEY) \
      deploy/cert-manager-webhook-he > "$(OUT)/rendered-manifest.yaml"

lint:
	helm lint \
	--kube-version 1.32.0 \
	deploy/cert-manager-webhook-he
