package service

import (
	"regexp"
	"strings"
)

// NL toll road registry — based on the official "Lijst met wegen"
// from vrachtwagenheffing.nl (effective 1 July 2026).
//
// Source: https://www.vrachtwagenheffing.nl/-/media/trucktol/website/
//         hier-gaat-u-vrachtwagenheffing-betalen/lijst-met-wegen.pdf
//
// The toll applies to:
//   - 39 A-wegen (motorways)
//   - 6 N-wegen (rijkswegen)
//   - 31 N-wegen (provinciale wegen)
//   - 4 provinciale A-wegen
//   - 7 gemeentelijke wegsegmenten (4 municipalities)

// nlTollRoads contains all road numbers where vrachtwagenheffing applies.
// Keys are normalized: "A1", "A2", "N279", etc.
var nlTollRoads = map[string]bool{
	// ── Rijkswegen: A-wegen ─────────────────────────────────
	"A1": true, "A2": true, "A4": true, "A5": true, "A6": true,
	"A7": true, "A8": true, "A9": true, "A10": true, "A12": true,
	"A13": true, "A15": true, "A16": true, "A17": true, "A18": true,
	"A20": true, "A22": true, "A27": true, "A28": true, "A29": true,
	"A30": true, "A32": true, "A35": true, "A37": true, "A38": true,
	"A44": true, "A50": true, "A58": true, "A59": true, "A65": true,
	"A67": true, "A73": true, "A74": true, "A76": true, "A77": true,
	"A200": true, "A205": true, "A208": true, "A838": true,

	// ── Provinciale A-wegen ─────────────────────────────────
	"A256": true, "A325": true, "A326": true, "A348": true,

	// ── Rijkswegen: N-wegen ─────────────────────────────────
	"N2": true, "N11": true, "N44": true, "N50": true, "N65": true, "N79": true,

	// ── Provinciale N-wegen ─────────────────────────────────
	"N201": true, "N207": true, "N209": true, "N212": true, "N214": true,
	"N221": true, "N225": true, "N230": true, "N235": true, "N237": true,
	"N244": true, "N246": true, "N247": true, "N260": true, "N263": true,
	"N268": true, "N278": true, "N279": true, "N280": true, "N281": true,
	"N282": true, "N285": true, "N321": true, "N322": true, "N323": true,
	"N324": true, "N325": true, "N401": true, "N470": true, "N640": true,
	"N641": true, "N781": true,
}

// nlMunicipalTollNames contains substrings of municipal road names
// that are also subject to vrachtwagenheffing.
var nlMunicipalTollNames = []string{
	"parallelroute",  // Rotterdam — Parallelroute A15
	"waterlinieweg",  // Utrecht
	"vlijmenseweg",   // 's-Hertogenbosch
	"noorderbrug",    // Maastricht
}

// nlRoadRefPattern matches road references like "A1", "N279", "A15/A20", "E35"
var nlRoadRefPattern = regexp.MustCompile(`(?i)\b([AN]\d{1,3})\b`)

// IsNLTollRoad checks whether any of the given street names correspond
// to a road in the official NL vrachtwagenheffing road network.
//
// Valhalla returns street names like:
//   - "A2"
//   - "A2/E25"
//   - "N279"
//   - "Rijksweg A2"
//   - "Waterlinieweg"
func IsNLTollRoad(streetNames []string) bool {
	for _, name := range streetNames {
		upper := strings.ToUpper(name)

		// Check road references (A1, N279, etc.)
		matches := nlRoadRefPattern.FindAllString(upper, -1)
		for _, m := range matches {
			if nlTollRoads[strings.ToUpper(m)] {
				return true
			}
		}

		// Check municipal road names
		lower := strings.ToLower(name)
		for _, municipal := range nlMunicipalTollNames {
			if strings.Contains(lower, municipal) {
				return true
			}
		}
	}
	return false
}
