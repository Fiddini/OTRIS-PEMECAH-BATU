#!/usr/bin/env bash
#
# E2E-01b: Full cross-repository integration test orchestrator
#
# Invokes:
# 1. WIJOYO producer: TestProduceE2E01b (writes profile.json + attestation + public key to tmpdir)
# 2. EDGE consumer: TestConsumeE2E01b (reads artifacts, executes full pipeline, verifies 12 assertions)
#
# Exit code:
#   0 = both producer and consumer passed
#   1 = producer failed
#   2 = consumer failed
#   3 = environment setup failed

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

WIJOYO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EDGE_DIR="/Users/macbook/Documents/PLUG_PLAY_15MENIT/otris-edge-agent"

echo -e "${YELLOW}[E2E-01b] Starting cross-repository integration test${NC}"
echo "  WIJOYO: $WIJOYO_DIR"
echo "  EDGE:   $EDGE_DIR"

# Create ephemeral artifacts directory
ARTIFACTS_DIR=$(mktemp -d)
trap "rm -rf $ARTIFACTS_DIR" EXIT

echo -e "${YELLOW}[E2E-01b] Artifacts directory: $ARTIFACTS_DIR${NC}"

# Phase A: WIJOYO Producer
echo -e "${YELLOW}[E2E-01b Phase A] Running WIJOYO producer (TestProduceE2E01b)${NC}"
if ! E2E_ARTIFACTS_DIR="$ARTIFACTS_DIR" go -C "$WIJOYO_DIR/OTRIS-PEMECAH-BATU-TANPA-JETSON" test -v ./internal/registry -run TestProduceE2E01b; then
	echo -e "${RED}[E2E-01b] Phase A FAILED: producer test did not complete${NC}"
	exit 1
fi
echo -e "${GREEN}[E2E-01b Phase A] PASSED${NC}"

# Read public key produced by WIJOYO
PUBLIC_KEY_HEX=$(cat "$ARTIFACTS_DIR/e2e-public-key.hex")
if [ -z "$PUBLIC_KEY_HEX" ]; then
	echo -e "${RED}[E2E-01b] Phase A produced empty public key${NC}"
	exit 3
fi
echo -e "${YELLOW}[E2E-01b] Public key loaded (hex length: ${#PUBLIC_KEY_HEX})${NC}"

# Verify artifacts were written
for artifact in profile.json registry_attestation.json e2e-public-key.hex; do
	if [ ! -f "$ARTIFACTS_DIR/$artifact" ]; then
		echo -e "${RED}[E2E-01b] Missing artifact: $artifact${NC}"
		exit 3
	fi
done
echo -e "${YELLOW}[E2E-01b] All artifacts present${NC}"

# Phase B: EDGE Consumer
echo -e "${YELLOW}[E2E-01b Phase B] Running EDGE consumer (TestConsumeE2E01b)${NC}"
if ! E2E_ARTIFACTS_DIR="$ARTIFACTS_DIR" E2E_PUBLIC_KEY_HEX="$PUBLIC_KEY_HEX" E2E_KEY_ID="e2e-test-issuer-01b" go -C "$EDGE_DIR" test -v ./cmd/edge -run TestConsumeE2E01b; then
	echo -e "${RED}[E2E-01b] Phase B FAILED: consumer test did not complete${NC}"
	exit 2
fi
echo -e "${GREEN}[E2E-01b Phase B] PASSED${NC}"

echo -e "${GREEN}[E2E-01b] ✓ INTEGRATION TEST PASSED (all 12 governance assertions verified)${NC}"
exit 0
