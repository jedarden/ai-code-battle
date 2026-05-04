#!/bin/bash
# Fix script for iad-acb R2 credentials corruption
# Problem: Values in OpenBao at secret/rs-manager/ai-code-battle/r2 are swapped/corrupted
# This script updates OpenBao with correct R2 credentials

set -e

KUBECONFIG="${KUBECONFIG:-/home/coding/.kube/iad-acb.kubeconfig}"
NAMESPACE="ai-code-battle"
SECRET_NAME="acb-r2-credentials"

# Default values (can be overridden via environment or prompts)
R2_ENDPOINT="${ACB_R2_ENDPOINT:-https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com}"
R2_BUCKET="${ACB_R2_BUCKET:-acb-data}"

echo "=== iad-acb R2 Credentials Fix ==="
echo ""
echo "This script fixes the corrupted R2 credentials in OpenBao."
echo ""

# Check if OpenBao is accessible
echo "Checking OpenBao connection..."
OPENBAO_ADDR="http://openbao.external-secrets.svc.cluster.local:8200"
if ! curl -s --connect-timeout 5 "$OPENBAO_ADDR/v1/sys/health" > /dev/null 2>&1; then
    echo "❌ Cannot reach OpenBao at $OPENBAO_ADDR"
    echo ""
    echo "Options:"
    echo "1. Create a SealedSecret instead (bypass OpenBao)"
    echo "2. Fix OpenBao connectivity first"
    echo ""
    read -p "Create SealedSecret? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        CREATE_SEALED_SECRET=true
    else
        echo "Exiting. Please fix OpenBao connectivity or provide R2 credentials for SealedSecret."
        exit 1
    fi
else
    echo "✓ OpenBao is reachable"
    CREATE_SEALED_SECRET=false
fi

# Prompt for R2 credentials
echo ""
echo "Enter R2 credentials (from Cloudflare Dashboard > R2 > acb-data > Settings > R2 API):"
echo ""

if [ -z "$ACB_R2_ACCESS_KEY" ]; then
    read -p "R2 Access Key ID (32 chars): " ACB_R2_ACCESS_KEY
else
    echo "Using ACB_R2_ACCESS_KEY from environment"
fi

if [ -z "$ACB_R2_SECRET_KEY" ]; then
    read -sp "R2 Secret Access Key (64 chars): " ACB_R2_SECRET_KEY
    echo
else
    echo "Using ACB_R2_SECRET_KEY from environment"
fi

# Validate inputs
if [ ${#ACB_R2_ACCESS_KEY} -lt 20 ]; then
    echo "❌ Access Key too short (expected ~32 chars)"
    exit 1
fi

if [ ${#ACB_R2_SECRET_KEY} -lt 40 ]; then
    echo "❌ Secret Key too short (expected ~64 chars)"
    exit 1
fi

echo ""
echo "=== Configuration ==="
echo "Endpoint:  $R2_ENDPOINT"
echo "Bucket:    $R2_BUCKET"
echo "Access Key: ${ACB_R2_ACCESS_KEY:0:8}..."
echo "Secret Key: ${ACB_R2_SECRET_KEY:0:8}..."
echo ""

if [ "$CREATE_SEALED_SECRET" = true ]; then
    echo "=== Creating SealedSecret ==="
    echo ""
    echo "Creating SealedSecret to bypass ESO..."

    # Create a temporary secret file
    TEMP_SECRET=$(mktemp)
    cat > "$TEMP_SECRET" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
  namespace: $NAMESPACE
type: Opaque
data:
  endpoint: $(echo -n "$R2_ENDPOINT" | base64 -w0)
  bucket: $(echo -n "$R2_BUCKET" | base64 -w0)
  access-key: $(echo -n "$ACB_R2_ACCESS_KEY" | base64 -w0)
  secret-key: $(echo -n "$ACB_R2_SECRET_KEY" | base64 -w0)
EOF

    # Seal it
    echo "Sealing secret..."
    kubectl --kubeconfig="$KUBECONFIG" delete secret $SECRET_NAME -n $NAMESPACE --ignore-not-found=true

    # Check if kubeseal is available
    if ! command -v kubeseal &> /dev/null; then
        echo "❌ kubeseal not found. Installing..."
        # Try to install from common locations
        if [ "$(uname -m)" = "x86_64" ]; then
            KUBESEAL_VERSION="0.24.0"
            wget -q "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz" -O /tmp/kubeseal.tar.gz
            tar -xzf /tmp/kubeseal.tar.gz -C /tmp kubeseal
            sudo install -m 755 /tmp/kubeseal /usr/local/bin/kubeseal
            rm /tmp/kubeseal.tar.gz /tmp/kubeseal
        else
            echo "Please install kubeseal manually:"
            echo "  https://github.com/bitnami-labs/sealed-secrets/releases"
            exit 1
        fi
    fi

    SEALED_SECRET=$(kubeseal --format=yaml < "$TEMP_SECRET")
    rm "$TEMP_SECRET"

    echo ""
    echo "=== SealedSecret Generated ==="
    echo ""
    echo "$SEALED_SECRET"
    echo ""
    echo "Apply this SealedSecret to the cluster:"
    echo "  echo '$SEALED_SECRET' | kubectl --kubeconfig=$KUBECONFIG apply -f -"
    echo ""
    echo "Then remove the ExternalSecret from declarative-config:"
    echo "  rm /home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-r2-credentials-externalsecret.yml"

else
    echo "=== Updating OpenBao Secret ==="
    echo ""
    echo "The script needs OpenBao admin access to update the secret."
    echo ""
    echo "Option A: Provide OpenBao root token"
    read -sp "OpenBao root token (leave empty to skip): " OPENBAO_TOKEN
    echo

    if [ -n "$OPENBAO_TOKEN" ]; then
        echo "Updating OpenBao secret at: secret/rs-manager/ai-code-battle/r2"

        # Use kubectl exec to access OpenBao
        OPENBAO_POD=$(kubectl --kubeconfig="$KUBECONFIG" get pods -n openbao -l app.kubernetes.io/name=openbao -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

        if [ -z "$OPENBAO_POD" ]; then
            echo "❌ Cannot find OpenBao pod in openbao namespace"
            echo "Trying direct API access..."

            # Try direct API access (requires network reachability)
            curl -s -X POST "$OPENBAO_ADDR/v1/auth/token/create" \
                -H "X-Vault-Token: $OPENBAO_TOKEN" \
                -H "Content-Type: application/json" \
                -d '{"policies": ["root"]}' > /dev/null 2>&1 || {
                    echo "❌ Cannot authenticate with OpenBao"
                    exit 1
                }
        fi

        # Update the secret via API
        RESPONSE=$(curl -s -X POST "$OPENBAO_ADDR/v1/secret/data/rs-manager/ai-code-battle/r2" \
            -H "X-Vault-Token: $OPENBAO_TOKEN" \
            -H "Content-Type: application/json" \
            -d "{
                \"data\": {
                    \"endpoint\": \"$R2_ENDPOINT\",
                    \"bucket\": \"$R2_BUCKET\",
                    \"access-key\": \"$ACB_R2_ACCESS_KEY\",
                    \"secret-key\": \"$ACB_R2_SECRET_KEY\"
                }
            }")

        if echo "$RESPONSE" | jq -e '.errors' > /dev/null 2>&1; then
            echo "❌ Failed to update OpenBao secret:"
            echo "$RESPONSE" | jq -r '.errors[]'
            exit 1
        else
            echo "✓ OpenBao secret updated successfully"
        fi

        # Force ESO to re-sync
        echo "Forcing ESO to re-sync..."
        kubectl --kubeconfig="$KUBECONFIG" annotate externalsecret $SECRET_NAME -n $NAMESPACE force-sync=$(date +%s) --overwrite

        echo "✓ ExternalSecret annotation added"
    else
        echo ""
        echo "=== Option B: Manual OpenBao Update ==="
        echo ""
        echo "Update the secret manually in OpenBao:"
        echo ""
        echo "  vault login <root-token>"
        echo "  vault kv put secret/rs-manager/ai-code-battle/r2 \\"
        echo "    endpoint=\"$R2_ENDPOINT\" \\"
        echo "    bucket=\"$R2_BUCKET\" \\"
        echo "    access-key=\"$ACB_R2_ACCESS_KEY\" \\"
        echo "    secret-key=\"$ACB_R2_SECRET_KEY\""
        echo ""
        echo "Then force ESO re-sync:"
        echo "  kubectl --kubeconfig=$KUBECONFIG annotate externalsecret $SECRET_NAME -n $NAMESPACE force-sync=\$(date +%s)"
    fi
fi

echo ""
echo "=== Verification ==="
echo ""
echo "After applying the fix, verify the secret:"
echo "  kubectl --kubeconfig=$KUBECONFIG get secret $SECRET_NAME -n $NAMESPACE -o json | jq -r '.data | map_values(@base64d)'"
echo ""
echo "Expected values:"
echo "  endpoint: $R2_ENDPOINT"
echo "  bucket: $R2_BUCKET"
echo "  access-key: $ACB_R2_ACCESS_KEY"
echo "  secret-key: <64-char secret key>"
echo ""
