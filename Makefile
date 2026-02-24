.PHONY: validate-kvm bootstrap-host fmt-check build-runtime-agent

validate-kvm:
	bash scripts/validate_kvm.sh

bootstrap-host:
	bash scripts/bootstrap_host.sh

fmt-check:
	sh -n scripts/validate_kvm.sh scripts/bootstrap_host.sh

build-runtime-agent:
	go build -o bin/runtime-agent ./cmd/runtime-agent
