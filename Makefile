GO ?= go
IMAGE ?= ghcr.io/contentways/cert-manager-webhook-poweradmin
TAG ?= dev

LOCALBIN ?= $(CURDIR)/bin
HELM ?= helm
CHART ?= deploy/charts/cert-manager-webhook-poweradmin
READMEGEN ?= npx --yes @bitnami/readme-generator-for-helm@2
E2E_DIR ?= $(CURDIR)/_e2e
# setup-envtest from the controller-runtime release in go.mod
ENVTEST_VERSION ?= release-0.24
ENVTEST_K8S_VERSION ?= 1.35.0

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build -trimpath -o $(LOCALBIN)/webhook .

.PHONY: test
test:
	$(GO) test -race ./...

.PHONY: lint
lint:
	golangci-lint run --build-tags conformance ./...

# Directory with etcd, kube-apiserver and kubectl; downloaded when unset.
ENVTEST_ASSETS ?= $(shell $(GO) run sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)

# Runs the cert-manager conformance suite against Poweradmin and PowerDNS
# started by hack/e2e/up.sh.
.PHONY: test-conformance
test-conformance:
	@assets="$(ENVTEST_ASSETS)"; \
	test -x "$$assets/kube-apiserver" || { echo "no envtest binaries in '$$assets'" >&2; exit 1; }; \
	E2E_DIR=$(E2E_DIR) hack/e2e/up.sh && \
	set -a && . $(E2E_DIR)/env && set +a && \
	TEST_ASSET_ETCD=$$assets/etcd \
	TEST_ASSET_KUBE_APISERVER=$$assets/kube-apiserver \
	TEST_ASSET_KUBECTL=$$assets/kubectl \
	$(GO) test -tags conformance -count=1 -run TestConformance -v . ; \
	status=$$?; E2E_DIR=$(E2E_DIR) hack/e2e/down.sh; exit $$status

.PHONY: image
image:
	docker build --build-arg VERSION=$(TAG) -t $(IMAGE):$(TAG) .

.PHONY: chart-lint
chart-lint:
	$(HELM) dependency build $(CHART)
	$(HELM) lint --strict deploy/charts/common
	@for f in $(CHART)/ci/*.yaml; do echo "== $$f"; $(HELM) lint --strict $(CHART) -f $$f || exit 1; done

# Regenerates the parameter table in the chart README and the values schema from the @param comments
.PHONY: chart-readme
chart-readme:
	$(READMEGEN) -v $(CHART)/values.yaml -r $(CHART)/README.md -s $(CHART)/values.schema.json
	hack/schema-overrides.py $(CHART)/values.schema.json $(CHART)/schema-overrides.json

.PHONY: clean
clean:
	-E2E_DIR=$(E2E_DIR) hack/e2e/down.sh
	chmod -R u+w $(LOCALBIN) 2>/dev/null || true
	rm -rf $(LOCALBIN) $(E2E_DIR) $(CHART)/charts .dist
