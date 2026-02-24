.PHONY: validate-kvm bootstrap-host fmt-check build-runtime-agent install-services e2e

validate-kvm:
	bash scripts/validate_kvm.sh

bootstrap-host:
	bash scripts/bootstrap_host.sh

fmt-check:
	sh -n scripts/validate_kvm.sh scripts/bootstrap_host.sh scripts/setup_networking.sh scripts/install_networking_service.sh scripts/install_storage_service.sh scripts/install_runtime_agent_service.sh scripts/e2e_verify.sh

build-runtime-agent:
	go build -o bin/runtime-agent ./cmd/runtime-agent

install-services:
	bash scripts/install_networking_service.sh
	bash scripts/install_storage_service.sh
	bash scripts/install_runtime_agent_service.sh

e2e:
	bash scripts/e2e_verify.sh
