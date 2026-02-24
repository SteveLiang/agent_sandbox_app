.PHONY: validate-kvm bootstrap-host fmt-check

validate-kvm:
	bash scripts/validate_kvm.sh

bootstrap-host:
	bash scripts/bootstrap_host.sh

fmt-check:
	sh -n scripts/validate_kvm.sh scripts/bootstrap_host.sh
