#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Quick start: init git repo en push naar GitHub
# ============================================================================
#
# Gebruik:
#   chmod +x scripts/init-repo.sh
#   ./scripts/init-repo.sh
# ============================================================================

GITHUB_OWNER="${GITHUB_OWNER:-filogic}"
GITHUB_REPO="${GITHUB_REPO:-micro-services-valhalla}"

echo "Viatiq Routing — Git init"
echo ""

# Init repo als dat nog niet is gebeurd
if [ ! -d .git ]; then
  git init
  echo "Git repo geïnitialiseerd"
fi

# Voeg alles toe
git add -A
git commit -m "feat: initial Viatiq routing service

- Valhalla routing engine (Europe + Asia OSM data)
- .NET 8 API gateway with truck/hazmat profiles
- Toll calculation NL/BE/DE/FR (km-heffing per euro-klasse)
- CO2 emission calculation (GLEC v3 / ISO 14083, WTW)
- Docker Compose for local development
- Cloud Build pipelines (API deploy + weekly tile rebuild)
- Cloud Run deployment config
- GCP setup automation script"

# Remote toevoegen als die er nog niet is
if ! git remote | grep -q origin; then
  git remote add origin "git@github.com:filogic/micro-services-valhalla.git"
  echo "Remote origin toegevoegd: git@github.com:filogic/micro-services-valhalla.git"
fi

git branch -M main

echo ""
echo "Klaar! Push met:"
echo "  git push -u origin main"
echo ""
echo "Dit triggert automatisch een Cloud Build als de trigger is ingesteld."
