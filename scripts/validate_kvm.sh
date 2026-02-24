#!/usr/bin/env bash
set -euo pipefail

echo "== /dev/kvm =="
ls -l /dev/kvm

echo "== CPU virtualization flags count =="
egrep -c '(vmx|svm)' /proc/cpuinfo

echo "== kvm-ok =="
if command -v kvm-ok >/dev/null 2>&1; then
  kvm-ok
else
  echo "kvm-ok not found; install cpu-checker package"
  exit 1
fi

echo "== Kernel =="
uname -a

echo "KVM validation passed."
