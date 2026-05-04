#!/bin/bash
# Fix script for iad-acb ClusterSecretStore issue
# Problem: Orphaned openbao namespace/service causing DNS conflicts

set -e

KUBECONFIG="${KUBECONFIG:-/home/coding/.kube/iad-acb.kubeconfig}"

echo "=== Checking iad-acb cluster access ==="
if ! kubectl --kubeconfig="$KUBECONFIG" get namespace openbao >/dev/null 2>&1; then
    echo "✓ No openbao namespace found - nothing to clean up!"
    echo "The ClusterSecretStore should work correctly now."
    exit 0
fi

echo "⚠️  Found openbao namespace - checking if it's managed by ArgoCD..."

# Check if there's an ArgoCD Application for openbao in iad-acb
ARGOCD_APPS=$(kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
    get applications -n argocd -o json 2>/dev/null | \
    jq -r '.items[] | select(.spec.destination.server | contains("iad-acb")) | select(.spec.destination.namespace == "openbao") | .metadata.name')

if [ -n "$ARGOCD_APPS" ]; then
    echo "⚠️  openbao namespace is managed by ArgoCD (apps: $ARGOCD_APPS)"
    echo "Do NOT delete manually - update ArgoCD apps instead."
    echo "Check declarative-config for iad-acb openbao resources."
    exit 1
fi

echo "✓ openbao namespace is orphaned (not managed by ArgoCD)"
echo ""
echo "=== Orphaned openbao resources found ==="
kubectl --kubeconfig="$KUBECONFIG" get all -n openbao 2>/dev/null || echo "No resources found"
echo ""

read -p "Delete orphaned openbao namespace? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting openbao namespace..."
    kubectl --kubeconfig="$KUBECONFIG" delete namespace openbao
    echo "✓ Deleted openbao namespace"
fi

echo ""
echo "=== Verifying ClusterSecretStore ==="
kubectl --kubeconfig="$KUBECONFIG" get clustersecretstore openbao -o yaml

echo ""
echo "=== Checking ExternalSecrets status ==="
for es in acb-evolver-secrets acb-armor acb-docker-hub; do
    echo -n "$es: "
    kubectl --kubeconfig="$KUBECONFIG" get externalsecret "$es" -n ai-code-battle -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Not found"
done

echo ""
echo "Done! Check acb-evolver pod status:"
kubectl --kubeconfig="$KUBECONFIG" get pods -n ai-code-battle -l app=acb-evolver
