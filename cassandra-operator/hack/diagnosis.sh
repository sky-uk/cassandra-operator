#!/usr/bin/env bash
set -e

function kindWorkerDiagnosis {
  local worker=$1
  echo ""
  echo "=== Dmesg ==="
  docker exec $worker dmesg -H

  echo ""
  echo "=== Journalctl ==="
  docker exec $worker journalctl -xe --since "5 min ago"
}

echo "==========================="
echo "==== Diagnosising host ===="
echo "==========================="
echo ""
echo "=== Dmesg ==="
dmesg -H

echo ""
echo "=== Journalctl ==="
journalctl -xe --since "5 min ago"

echo "==========================="
echo "==== Diagnosising kind ===="
echo "==========================="
for worker in kind-worker kind-worker2 kind-worker3 kind-worker4
do
  echo "==== Diagnosis kind worker ${worker}"
  kindWorkerDiagnosis ${worker}
  echo ""
done