#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Viatiq Routing Service — GCP Setup
# ============================================================================
#
# Architectuur:
#   Go Cloud Function (API) → Cloud Run (Valhalla routing engine)
#   Cloud Scheduler → Cloud Build → GCS (wekelijkse tile rebuild)
#
# Gebruik:
#   export GCP_PROJECT_ID=filogic-viatiq
#   export GITHUB_OWNER=filogic
#   ./scripts/setup-gcp.sh
# ============================================================================

PROJECT_ID="${GCP_PROJECT_ID:-filogic-viatiq}"
REGION="europe-west4"
GITHUB_OWNER="${GITHUB_OWNER:-filogic}"
GITHUB_REPO="${GITHUB_REPO:-micro-services-valhalla}"
TILES_BUCKET="${PROJECT_ID}-valhalla-tiles"
VALHALLA_SERVICE="valhalla"
FUNCTION_NAME="prod-filogic-services-route"
SCHEDULER_JOB="valhalla-tile-rebuild"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; RED='\033[0;31m'; NC='\033[0m'
log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
info() { echo -e "${BLUE}[→]${NC} $1"; }
err()  { echo -e "${RED}[✗]${NC} $1"; exit 1; }

echo ""
echo "  Viatiq Routing — GCP Setup"
echo "  Project: ${PROJECT_ID} | Region: ${REGION}"
echo ""

gcloud config set project "${PROJECT_ID}" 2>/dev/null

# ── 1. APIs ───────────────────────────────────────────────────────────────────
info "APIs enablen..."
gcloud services enable \
  cloudfunctions.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  cloudscheduler.googleapis.com \
  storage.googleapis.com \
  --quiet
log "APIs enabled"

# ── 2. GCS bucket voor tiles ─────────────────────────────────────────────────
info "GCS bucket aanmaken..."
if gsutil ls -b "gs://${TILES_BUCKET}" 2>/dev/null; then
  warn "Bucket bestaat al"
else
  gsutil mb -p "${PROJECT_ID}" -l "${REGION}" -b on "gs://${TILES_BUCKET}"
  log "Bucket ${TILES_BUCKET} aangemaakt"
fi

# ── 3. IAM ────────────────────────────────────────────────────────────────────
info "IAM permissions..."
PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format="value(projectNumber)")
CB_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"

for role in roles/run.admin roles/cloudfunctions.admin roles/storage.objectAdmin roles/iam.serviceAccountUser; do
  gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
    --member="serviceAccount:${CB_SA}" --role="${role}" --quiet >/dev/null
done
log "IAM geconfigureerd"

# ── 4. Cloud Run — Valhalla ───────────────────────────────────────────────────
info "Valhalla Cloud Run service deployen..."
gcloud run deploy "${VALHALLA_SERVICE}" \
  --image="ghcr.io/valhalla/valhalla-scripted:latest" \
  --region="${REGION}" \
  --memory=16Gi --cpu=4 \
  --min-instances=1 --max-instances=4 \
  --port=8002 \
  --no-allow-unauthenticated \
  --cpu-boost \
  --execution-environment=gen2 \
  --set-env-vars="serve_tiles=True,use_tiles_ignore_pbf=True,server_threads=4" \
  --timeout=300 \
  --quiet

VALHALLA_URL=$(gcloud run services describe "${VALHALLA_SERVICE}" \
  --region="${REGION}" --format="value(status.url)")
log "Valhalla: ${VALHALLA_URL}"

# ── 5. Cloud Function — API ──────────────────────────────────────────────────
info "Cloud Function deployen..."
gcloud functions deploy "${FUNCTION_NAME}" \
  --gen2 \
  --runtime=go122 \
  --region="${REGION}" \
  --source=. \
  --entry-point=Route \
  --trigger-http \
  --allow-unauthenticated \
  --memory=128Mi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=100 \
  --timeout=30s \
  --set-env-vars="VALHALLA_BASE_URL=${VALHALLA_URL},DATA_PATH=./internal/data" \
  --quiet

FUNCTION_URL=$(gcloud functions describe "${FUNCTION_NAME}" \
  --region="${REGION}" --format="value(serviceConfig.uri)")
log "Function: ${FUNCTION_URL}"

# ── 6. GitHub trigger ─────────────────────────────────────────────────────────
echo ""
warn "══════════════════════════════════════════════════════════"
warn "  HANDMATIGE STAP: GitHub koppeling"
warn "══════════════════════════════════════════════════════════"
echo ""
echo "  1. Ga naar: https://console.cloud.google.com/cloud-build/triggers/connect?project=${PROJECT_ID}"
echo "  2. Kies 'GitHub (Cloud Build GitHub App)'"
echo "  3. Selecteer repo: ${GITHUB_OWNER}/${GITHUB_REPO}"
echo ""
echo "  Druk op ENTER zodra klaar..."
read -r

gcloud builds triggers create github \
  --name="${FUNCTION_NAME}-push" \
  --region="${REGION}" \
  --repo-name="${GITHUB_REPO}" \
  --repo-owner="${GITHUB_OWNER}" \
  --branch-pattern="^main$" \
  --build-config="scripts/cloudbuild-api.yaml" \
  --substitutions="_REGION=${REGION},_VALHALLA_URL=${VALHALLA_URL}" \
  --description="Deploy Cloud Function on push to main" \
  2>/dev/null || warn "Trigger bestaat al"
log "GitHub trigger aangemaakt"

# ── 7. Cloud Scheduler — tiles ────────────────────────────────────────────────
info "Tile build config uploaden + scheduler aanmaken..."
gsutil cp scripts/cloudbuild-tiles.yaml "gs://${TILES_BUCKET}/cloudbuild-tiles.yaml"

gcloud scheduler jobs create http "${SCHEDULER_JOB}" \
  --location="${REGION}" \
  --schedule="0 2 * * 0" \
  --time-zone="Europe/Amsterdam" \
  --uri="https://cloudbuild.googleapis.com/v1/projects/${PROJECT_ID}/locations/${REGION}/builds" \
  --message-body="{\"source\":{\"storageSource\":{\"bucket\":\"${TILES_BUCKET}\",\"object\":\"cloudbuild-tiles.yaml\"}},\"substitutions\":{\"_TILES_BUCKET\":\"${TILES_BUCKET}\"}}" \
  --oauth-service-account-email="${CB_SA}" \
  --description="Wekelijkse tile rebuild (zo 02:00 CET)" \
  --quiet 2>/dev/null || warn "Scheduler job bestaat al"
log "Scheduler aangemaakt"

# ── Klaar ─────────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════════"
echo -e "  ${GREEN}Setup compleet!${NC}"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "  Cloud Function (API): ${FUNCTION_URL}"
echo "  Cloud Run (Valhalla): ${VALHALLA_URL}"
echo "  GCS (tiles):          gs://${TILES_BUCKET}"
echo "  Scheduler:            zondag 02:00 CET"
echo ""
echo "  Volgende stappen:"
echo "    1. Eerste tile build: gcloud scheduler jobs run ${SCHEDULER_JOB} --location=${REGION}"
echo "    2. Test: curl -X POST ${FUNCTION_URL}/api/v1/route \\"
echo "         -H 'Content-Type: application/json' \\"
echo "         -d '{\"origin\":{\"lat\":52.09,\"lon\":5.12},\"destination\":{\"lat\":51.44,\"lon\":5.47}}'"
echo ""
