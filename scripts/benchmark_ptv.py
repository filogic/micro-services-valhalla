#!/usr/bin/env python3
"""Benchmark Valhalla toll against PTV for a random sample of stored queries.

Pure measurement, no judgment. It:
  1. Lists captured queries in gs://$QUERY_BUCKET/queries/** and random-samples N.
  2. Re-runs each through Valhalla (current tariffs) AND PTV for the same
     origin/destination/waypoints + vehicle.
  3. Computes per-country and total toll deviation, plus a tolled-distance
     ratio (Valhalla/PTV) where PTV exposes it — so the daily tuning routine
     can tell a *rate* error from a *road-matching* error.
  4. Writes a structured raw.json (+ prints it to stdout) to
     gs://$QUERY_BUCKET/benchmarks/<date>/raw.json.

The judgment/tuning/email lives in benchmark/MASTER_PROMPT.md, not here.

Env:
  QUERY_BUCKET     default filogic-opentms-valhalla-queries
  VALHALLA_URL     default the prod route function
  PTV_API_KEY      else read from Secret Manager secret `ptv-api-key`
  PTV_TOLL_TIME    default 2026-07-15T08:00:00.000Z (NL heffing active regime)
  SAMPLE_N         default 10
  RUN_DATE         default today (UTC)
"""

import datetime
import json
import os
import random
import subprocess
import sys
from collections import defaultdict

BUCKET = os.environ.get("QUERY_BUCKET", "filogic-opentms-valhalla-queries")
VALHALLA_URL = os.environ.get(
    "VALHALLA_URL",
    "https://europe-west4-filogic-opentms.cloudfunctions.net/prod-filogic-services-route",
).rstrip("/")
PTV_BASE = "https://api.myptv.com/routing/v1/routes"
# Pin to the all-tolls-active regime so PTV applies NL vrachtwagenheffing
# (starts 1 Jul 2026) just like Valhalla does — otherwise NL deviation is bogus.
TOLL_TIME = os.environ.get("PTV_TOLL_TIME", "2026-07-15T08:00:00.000Z")
SAMPLE_N = int(os.environ.get("SAMPLE_N", "10"))
RUN_DATE = os.environ.get("RUN_DATE") or datetime.datetime.utcnow().strftime("%Y-%m-%d")
HTTP_TIMEOUT = 120

# our EuroClass -> PTV emissionStandard
EURO_TO_PTV = {
    "EURO_0": "EURO_0", "EURO_I": "EURO_1", "EURO_II": "EURO_2", "EURO_III": "EURO_3",
    "EURO_IV": "EURO_4", "EURO_V": "EURO_5", "EURO_VI": "EURO_6", "EURO_VI_E": "EURO_6",
}


def sh(args, stdin=None):
    return subprocess.run(args, input=stdin, capture_output=True, text=True, timeout=180)


def ptv_key():
    key = os.environ.get("PTV_API_KEY")
    if key:
        return key.strip()
    r = sh(["gcloud", "secrets", "versions", "access", "latest", "--secret=ptv-api-key"])
    if r.returncode != 0:
        sys.exit("cannot read ptv-api-key secret: " + r.stderr)
    return r.stdout.strip()


def list_query_objects():
    r = sh(["gcloud", "storage", "ls", "--recursive", f"gs://{BUCKET}/queries/**"])
    if r.returncode != 0:
        return []
    return [ln.strip() for ln in r.stdout.splitlines() if ln.strip().endswith(".json")]


def read_json(gs_uri):
    r = sh(["gcloud", "storage", "cat", gs_uri])
    if r.returncode != 0:
        raise RuntimeError("read failed: " + r.stderr.strip())
    return json.loads(r.stdout)


def write_object(gs_uri, text):
    r = sh(["gcloud", "storage", "cp", "-", gs_uri], stdin=text)
    if r.returncode != 0:
        raise RuntimeError("write failed: " + r.stderr.strip())


def curl_json(args):
    # Shell out to curl so we use the system CA store (robust headless; avoids
    # Python SSL/cert issues across environments).
    r = sh(["curl", "-s", "--max-time", str(HTTP_TIMEOUT)] + args)
    if r.returncode != 0:
        raise RuntimeError("curl failed: " + (r.stderr.strip() or "rc=%d" % r.returncode))
    try:
        return json.loads(r.stdout)
    except json.JSONDecodeError:
        raise RuntimeError("non-JSON response: " + r.stdout[:200])


def valhalla_toll(query):
    payload = {"origin": query["origin"], "destination": query["destination"]}
    if query.get("waypoints"):
        payload["waypoints"] = query["waypoints"]
    if query.get("vehicle"):
        payload["vehicle"] = query["vehicle"]
    d = curl_json(["-X", "POST", VALHALLA_URL + "/api/v1/route",
                   "-H", "Content-Type: application/json", "-d", json.dumps(payload)])
    toll = d.get("toll", {})
    by_country = {
        c["country"]: {"cost": c.get("cost", 0.0), "tolledKm": (c.get("tolledDistance") or 0) / 1000.0}
        for c in (toll.get("byCountry") or [])
    }
    return {"total": toll.get("totalCost", 0.0), "byCountry": by_country,
            "distanceKm": d.get("route", {}).get("distance", 0) / 1000.0}


def ptv_toll(query, key):
    points = [query["origin"]] + (query.get("waypoints") or []) + [query["destination"]]
    vehicle = query.get("vehicle") or {}
    weight_kg = int(round((vehicle.get("weight") or 40) * 1000))
    emission = EURO_TO_PTV.get(vehicle.get("euroClass") or "EURO_VI", "EURO_6")

    # PTV rejects duplicate params, so `results` is a single comma-joined value.
    # (TOLL_SECTIONS returns no per-section distance here, so we don't request
    # it — rate-vs-road is decided from total distance + outcome instead.)
    args = ["-G", PTV_BASE, "-H", f"apiKey: {key}", "-H", "Accept: application/json",
            "--data-urlencode", "profile=EUR_TRAILER_TRUCK",
            "--data-urlencode", "results=TOLL_COSTS",
            "--data-urlencode", "options[trafficMode]=AVERAGE",
            "--data-urlencode", f"options[tollTime]={TOLL_TIME}",
            "--data-urlencode", f"vehicle[totalPermittedWeight]={weight_kg}",
            "--data-urlencode", f"vehicle[emissionStandard]={emission}"]
    for p in points:
        args += ["--data-urlencode", f"waypoints={p['lat']},{p['lon']}"]
    d = curl_json(args)

    costs = d.get("toll", {}).get("costs", {})
    prices = costs.get("prices") or [{}]
    by_country = {c["countryCode"]: {"cost": c.get("price", {}).get("price", 0.0), "tolledKm": None}
                  for c in costs.get("countries", [])}

    # Per-country tolled distance from sections, when PTV provides a distance.
    sec_dist, have_dist = defaultdict(float), False
    for s in d.get("toll", {}).get("sections", []):
        cc = s.get("countryCode")
        for dk in ("distance", "length", "tollDistance"):
            v = s.get(dk)
            if isinstance(v, (int, float)):
                sec_dist[cc] += v
                have_dist = True
                break
    if have_dist:
        for cc, meters in sec_dist.items():
            by_country.setdefault(cc, {"cost": 0.0})["tolledKm"] = meters / 1000.0

    return {"total": prices[0].get("price", 0.0), "byCountry": by_country,
            "distanceKm": d.get("distance", 0) / 1000.0, "violated": d.get("violated", False)}


def pct_dev(v, p):
    if p > 0:
        return (v - p) / p * 100.0
    return 0.0 if v == 0 else None  # None = PTV says €0 (e.g. country not tolled), can't take a ratio


def compare(val, ptv):
    rows = []
    for cc in sorted(set(val["byCountry"]) | set(ptv["byCountry"])):
        vc, pc = val["byCountry"].get(cc, {}), ptv["byCountry"].get(cc, {})
        vcost, pcost = vc.get("cost", 0.0), pc.get("cost", 0.0)
        vkm, pkm = vc.get("tolledKm"), pc.get("tolledKm")
        ratio = round(vkm / pkm, 3) if (vkm and pkm) else None
        dev = pct_dev(vcost, pcost)
        rows.append({
            "country": cc,
            "valhallaCost": round(vcost, 2), "ptvCost": round(pcost, 2),
            "absDiff": round(vcost - pcost, 2),
            "pctDev": round(dev, 1) if dev is not None else None,
            "valhallaTolledKm": round(vkm, 1) if vkm else None,
            "ptvTolledKm": round(pkm, 1) if pkm else None,
            "tolledDistRatio": ratio,
        })
    tdev = pct_dev(val["total"], ptv["total"])
    return {
        "valhallaTotal": round(val["total"], 2), "ptvTotal": round(ptv["total"], 2),
        "totalPctDev": round(tdev, 1) if tdev is not None else None,
        "ptvViolated": ptv.get("violated"),
        "valhallaDistanceKm": round(val.get("distanceKm", 0), 1),
        "ptvDistanceKm": round(ptv.get("distanceKm", 0), 1),
        "countries": rows,
    }


def main():
    key = ptv_key()
    objs = list_query_objects()
    sample = random.sample(objs, min(SAMPLE_N, len(objs))) if objs else []

    runs = []
    for gs in sample:
        try:
            q = read_json(gs)
            runs.append({"query": gs, "od": [q["origin"], q["destination"]],
                         "vehicle": q.get("vehicle"),  # so the routine knows which rate cell to tune
                         "cmp": compare(valhalla_toll(q), ptv_toll(q, key))})
        except Exception as e:  # noqa: BLE001 — one bad query must not sink the run
            runs.append({"query": gs, "error": str(e)})

    # Per-country aggregate across the sample.
    agg = defaultdict(lambda: {"n": 0, "sumPct": 0.0, "sumAbs": 0.0, "ratios": []})
    for r in runs:
        for c in r.get("cmp", {}).get("countries", []):
            if isinstance(c["pctDev"], (int, float)):
                a = agg[c["country"]]
                a["n"] += 1
                a["sumPct"] += c["pctDev"]
                a["sumAbs"] += abs(c["absDiff"])
                if c["tolledDistRatio"]:
                    a["ratios"].append(c["tolledDistRatio"])
    per_country = {
        cc: {
            "n": a["n"],
            "meanPctDev": round(a["sumPct"] / a["n"], 1),
            "meanAbsDiff": round(a["sumAbs"] / a["n"], 2),
            "meanDistRatio": round(sum(a["ratios"]) / len(a["ratios"]), 3) if a["ratios"] else None,
        }
        for cc, a in agg.items()
    }
    total_devs = [r["cmp"]["totalPctDev"] for r in runs
                  if r.get("cmp", {}).get("totalPctDev") is not None]

    out = {
        "date": RUN_DATE,
        "tollTime": TOLL_TIME,
        "sampleCount": len(sample),
        "available": len(objs),
        "meanTotalPctDev": round(sum(total_devs) / len(total_devs), 1) if total_devs else None,
        "perCountry": per_country,
        "runs": runs,
    }
    text = json.dumps(out, indent=2)
    if objs:
        try:
            write_object(f"gs://{BUCKET}/benchmarks/{RUN_DATE}/raw.json", text)
        except Exception as e:  # noqa: BLE001
            print(f"WARN: could not write results: {e}", file=sys.stderr)
    print(text)


if __name__ == "__main__":
    main()
