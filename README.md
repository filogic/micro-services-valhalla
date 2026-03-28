# Viatiq Routing Service

Multimodal routing API met tolkosten en CO₂-berekening voor Europa en Azië.
Gebouwd op Valhalla (open source routing engine) + OpenStreetMap data.

## Quick Start

### Prerequisites
- Docker + Docker Compose
- Go 1.22+ (voor lokale development)
- **Minimaal 32 GB RAM** (Valhalla tiles Europa + Azië)
- **~100 GB vrije schijfruimte** (PBF downloads + tile build)

### Starten met Docker Compose

```bash
# Start Valhalla + API
# EERSTE KEER: downloads Europe (~28 GB) + Asia (~12 GB) PBF
# en bouwt tiles. Dit duurt 2-4 uur op een moderne machine met SSD.
docker compose up -d

# Volg de tile build progress
docker logs -f viatiq-valhalla

# Zodra je ziet: "Tile extract successfully loaded" → ready
# Bij herstart worden bestaande tiles direct geladen (~60-90s)
```

### Test request (Utrecht → Eindhoven, vrachtwagen)

```bash
curl -X POST http://localhost:5000/api/v1/route \
  -H "Content-Type: application/json" \
  -d '{
    "origin": { "lat": 52.0907, "lon": 5.1214 },
    "destination": { "lat": 51.4416, "lon": 5.4697 },
    "vehicle": {
      "height": 4.0,
      "weight": 40.0,
      "length": 16.5,
      "axles": 5,
      "euroClass": "EURO_VI",
      "fuelType": "Diesel"
    },
    "cargo": {
      "weightTonnes": 12.0
    }
  }'
```

### Test request (personenauto — geen vehicle specs nodig)

```bash
curl -X POST http://localhost:5000/api/v1/route \
  -H "Content-Type: application/json" \
  -d '{
    "origin": { "lat": 52.0907, "lon": 5.1214 },
    "destination": { "lat": 51.4416, "lon": 5.4697 }
  }'
```

### Response voorbeeld

```json
{
  "route": {
    "distance": 98400,
    "duration": 4320,
    "polyline": "...",
    "vehicle": {
      "height": 4.0,
      "weight": 40.0,
      "length": 16.5,
      "axles": 5,
      "euroClass": "EURO_VI",
      "fuelType": "Diesel"
    },
    "legs": [{ "distance": 98400, "duration": 4320, "summary": "98.4 km, 01:12" }]
  },
  "carbonFootprint": {
    "totalKgCO2e": 95.6,
    "gCO2ePerTkm": 81.0,
    "methodology": "GLECv3/ISO14083",
    "scope": "WTW",
    "factors": {
      "emissionFactor": 3.24,
      "fuelConsumption": 0.30,
      "loadFactor": 0.6
    }
  },
  "toll": {
    "totalCost": 10.81,
    "currency": "EUR",
    "segments": [
      {
        "country": "NL",
        "operator": "Vrachtwagenheffing",
        "system": "Distance",
        "distance": 68880,
        "cost": 10.81,
        "ratePerKm": 0.157
      }
    ]
  }
}
```

## Architectuur

```
Client → Go Cloud Function → Valhalla Cloud Run (route) → Toll lookup → CO₂ calc → Response
         (scales to zero)       (always-on, 16GB)            (in-memory)   (pure math)
                                       │
                                       └── OSM tiles (GCS bucket, wekelijks rebuilt)
```

Zero dependencies. Geen framework (behalve functions-framework voor local dev).
stdlib `net/http` + `encoding/json`. Cold start: <100ms.

## Routing Logic

Geen `profile` veld — de vehicle specs bepalen de routing:

| Request | Valhalla costing | Gedrag |
|---------|-----------------|--------|
| Geen `vehicle` | `auto` | Standaard personenauto |
| `vehicle.height: 3.2` | `truck` | Vermijdt wegen met maxheight < 3.2m |
| `vehicle.weight: 40` | `truck` | Vermijdt wegen met maxweight < 40t |
| `vehicle.weight: 7.5, height: 3.0` | `truck` | Combineert alle restricties |
| `vehicle.hazmat: true` | `truck` + hazmat | + vermijdt tunnels, waterbescherming |

Zodra er fysieke parameters (height/weight/length/width/axles/axleLoad) of `hazmat` worden meegegeven, schakelt de routing automatisch over naar truck costing.

## Toll Coverage (V1)

| Land | Systeem | Operator | Scope |
|------|---------|----------|-------|
| NL   | km-heffing | Vrachtwagenheffing | >3.5t, per euro-klasse |
| DE   | km-heffing | Toll Collect | >7.5t, per assen + euro-klasse + CO₂ |
| BE   | km-heffing | Viapass | >3.5t, per gewicht + euro-klasse |
| FR   | per-traject | Autoroutes | >3.5t, vereenvoudigd gemiddelde |

## CO₂ Methodology

GLEC Framework v3 / ISO 14083 compliant:
- **Scope**: Well-to-Wheel (WTW = TTW + upstream)
- **Tier 1**: GLEC default emission factors (standaard)
- **Tier 2**: Override `fuelConsumption` in request voor carrier-specifiek
- **Output**: kg CO₂e totaal + g CO₂e/tkm

## Coverage

- **Routing**: Heel Europa + Azië (OpenStreetMap via Geofabrik)
- **Truck/Hazmat**: Alle landen waar OSM HGV-tags beschikbaar zijn
- **Tolkosten V1**: NL, DE, BE, FR (km-heffing per euro-klasse)
- **CO₂**: Wereldwijd (GLEC v3 defaults, voertuig-onafhankelijk)

## Deployment

**API** — Cloud Function (2nd gen), schaalt naar nul:
```bash
gcloud functions deploy viatiq-route \
  --gen2 --runtime=go122 --region=europe-west4 \
  --source=. --entry-point=Route \
  --trigger-http --allow-unauthenticated \
  --memory=128Mi --min-instances=0 --max-instances=100
```

**Valhalla** — Cloud Run, altijd aan (tiles in geheugen):
```bash
gcloud run deploy viatiq-valhalla \
  --image=ghcr.io/valhalla/valhalla-scripted:latest \
  --region=europe-west4 \
  --memory=16Gi --cpu=4 \
  --min-instances=1 --max-instances=4 --cpu-boost
```

**Lokaal testen:**
```bash
docker compose up -d          # start Valhalla
go run ./cmd/local            # start API lokaal op :8080
```

Tiles worden wekelijks gebouwd via Cloud Build + Cloud Scheduler.

## Roadmap

- [ ] V1.0: EU+Asia routing + NL/BE/DE/FR toll + CO₂ (MVP)
- [ ] V1.1: Admin boundary detection per Valhalla edge → exacte country distances
- [ ] V1.2: FR per-section tolling (péage-punt matching)
- [ ] V1.3: Vignette landen (AT, CH, HU, CZ, SK, BG, RO, SI)
- [ ] V2.0: TollGuru SDK integratie voor overige EU-landen
- [ ] V2.1: Elektrisch voertuig routing (range, laadstops)
- [ ] V2.2: Country-specifieke CO₂ grid-mix voor electric (kWh → CO₂e)
- [ ] V3.0: Multi-region deployment (europe-west4 + asia-east2)
