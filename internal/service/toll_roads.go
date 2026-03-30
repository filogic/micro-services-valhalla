package service

import (
	"regexp"
	"strings"
)

// TollRoadRegistry determines whether a road segment is tolled based on
// the street names returned by Valhalla. Each country has its own matching
// rules, ranging from simple prefix matching (DE: all A/B roads) to exact
// road-number matching (NL: official vrachtwagenheffing road list).
//
// Sources:
//   NL: vrachtwagenheffing.nl — "Lijst met wegen" (effective 1 Jul 2026)
//   DE: Toll Collect — all Bundesfernstraßen (Autobahn + Bundesstraßen)
//   BE: Viapass — all motorways + selected N-roads
//   FR: Autoroutes à péage — all A-roads
//   AT: ASFINAG GO-Maut — all Autobahnen (A) + Schnellstraßen (S)
//   CH: LSVA — all roads (100%)
//   IT: Autostrade per l'Italia — all A-roads
//   ES: Autopistas de peaje — AP-roads
//   PL: e-TOLL — autostrady (A) + drogi ekspresowe (S)
//   CZ: Electronic toll — dálnice (D) + selected class I roads
//   HU: HU-GO — gyorsforgalmi utak (M) + selected main roads
//   SK: Emyto — diaľnice (D) + rýchlostné cesty (R)
//   SI: DarsGo — avtoceste (A) + hitre ceste (H)
//   PT: Via Verde — autoestradas (A)
//   BG: Toll system — avtomаgistrali (A) + selected I-class roads
//   RO: RO e-Toll — autostrăzi (A) + selected drumuri naționale (DN)
//   HR: HAC — autoceste (A)
//   SE: Infrastrukturavgift — specific bridges/passages only
//   DK: Specific bridges (Storebælt, Øresund)

// IsTollRoad checks if the given street names indicate a tolled road
// in the specified country.
func IsTollRoad(country string, streetNames []string) bool {
	switch country {
	case "NL":
		return IsNLTollRoad(streetNames)
	case "CH":
		// Switzerland LSVA applies to ALL roads
		return true
	case "DE":
		return matchesPrefixes(streetNames, dePrefixes)
	case "FR":
		return matchesPrefixes(streetNames, frPrefixes)
	case "AT":
		return matchesPrefixes(streetNames, atPrefixes)
	case "IT":
		return matchesPrefixes(streetNames, itPrefixes)
	case "ES":
		return matchesSpain(streetNames)
	case "PL":
		return matchesPrefixes(streetNames, plPrefixes)
	case "CZ":
		return matchesCzechia(streetNames)
	case "HU":
		return matchesPrefixes(streetNames, huPrefixes)
	case "SK":
		return matchesPrefixes(streetNames, skPrefixes)
	case "SI":
		return matchesPrefixes(streetNames, siPrefixes)
	case "PT":
		return matchesPrefixes(streetNames, ptPrefixes)
	case "BG":
		return matchesPrefixes(streetNames, bgPrefixes)
	case "RO":
		return matchesRomania(streetNames)
	case "HR":
		return matchesPrefixes(streetNames, hrPrefixes)
	case "SE":
		return matchesSweden(streetNames)
	case "DK":
		return matchesDenmark(streetNames)
	case "BE":
		return matchesBelgium(streetNames)
	default:
		return false
	}
}

// ── Prefix sets per country ─────────────────────────────────────────

// roadRef extracts road references like "A1", "B42", "D1", "M7", "AP7" etc.
var roadRef = regexp.MustCompile(`(?i)\b([A-Z]{1,2}\d{1,4})\b`)

// Germany: ALL Autobahn (A) + ALL Bundesstraßen (B) are tolled for trucks ≥7.5t
// This covers ~52,000 km. Source: Toll Collect GmbH
var dePrefixes = []string{"A", "B"}

// France: Autoroutes (A-roads) are tolled
// ~9,000 km of autoroutes à péage. Source: ASFA
var frPrefixes = []string{"A"}

// Austria: Autobahnen (A) + Schnellstraßen (S) via GO-Maut
// ~2,200 km. Source: ASFINAG
var atPrefixes = []string{"A", "S"}

// Italy: Autostrade (A-roads) are tolled
// ~6,000 km. Source: Autostrade per l'Italia
var itPrefixes = []string{"A"}

// Poland: Autostrady (A) + Drogi ekspresowe (S)
// ~4,800 km. Source: e-TOLL / GDDKiA
var plPrefixes = []string{"A", "S"}

// Hungary: Gyorsforgalmi utak — Motorways (M) + main express roads
// ~2,100 km. Source: HU-GO / NMMA
var huPrefixes = []string{"M"}

// Slovakia: Diaľnice (D) + Rýchlostné cesty (R)
// ~800 km. Source: Emyto / NDS
var skPrefixes = []string{"D", "R"}

// Slovenia: Avtoceste (A) + Hitre ceste (H)
// ~770 km. Source: DarsGo / DARS
var siPrefixes = []string{"A", "H"}

// Portugal: Autoestradas (A-roads)
// ~3,000 km. Source: Via Verde
var ptPrefixes = []string{"A"}

// Bulgaria: Avtomagistrali (A) — motorways
// ~900 km. Source: Agency "Road Infrastructure"
var bgPrefixes = []string{"A"}

// Croatia: Autoceste (A-roads)
// ~1,300 km. Source: HAC / ARZ
var hrPrefixes = []string{"A"}

// ── Country-specific matchers ───────────────────────────────────────

// matchesPrefixes checks if any street name contains a road reference
// starting with one of the given prefixes.
func matchesPrefixes(streetNames []string, prefixes []string) bool {
	for _, name := range streetNames {
		refs := roadRef.FindAllString(strings.ToUpper(name), -1)
		for _, ref := range refs {
			for _, prefix := range prefixes {
				if strings.HasPrefix(ref, prefix) && len(ref) > len(prefix) {
					// Verify the char after the prefix is a digit
					rest := ref[len(prefix):]
					if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
						return true
					}
				}
			}
		}
	}
	return false
}

// Spain: AP-roads (autopistas de peaje) are tolled, A-roads (autovías) are free
func matchesSpain(streetNames []string) bool {
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		// AP7, AP2, AP68, etc. = toll
		if regexp.MustCompile(`\bAP\d{1,3}\b`).MatchString(upper) {
			return true
		}
		// Some C-roads near Barcelona are also tolled
		if regexp.MustCompile(`\bC-?3[123]\b`).MatchString(upper) {
			return true
		}
	}
	return false
}

// Czechia: Dálnice (D) + selected Silnice I. třídy (I/number)
var czTolledClassI = map[string]bool{
	"I/3": true, "I/6": true, "I/7": true, "I/8": true, "I/11": true,
	"I/13": true, "I/14": true, "I/16": true, "I/20": true, "I/21": true,
	"I/23": true, "I/25": true, "I/27": true, "I/33": true, "I/34": true,
	"I/35": true, "I/36": true, "I/37": true, "I/38": true, "I/43": true,
	"I/44": true, "I/46": true, "I/48": true, "I/49": true, "I/50": true,
	"I/52": true, "I/53": true, "I/55": true, "I/56": true, "I/57": true,
	"I/58": true, "I/67": true, "I/68": true, "I/69": true,
}

func matchesCzechia(streetNames []string) bool {
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		// D1, D2, D3, etc.
		if regexp.MustCompile(`\bD\d{1,2}\b`).MatchString(upper) {
			return true
		}
		// Class I roads: "I/35" or "35" referenced in czTolledClassI
		for road := range czTolledClassI {
			if strings.Contains(upper, strings.ToUpper(road)) {
				return true
			}
		}
	}
	return false
}

// Romania: Autostrăzi (A) + selected Drumuri Naționale (DN)
func matchesRomania(streetNames []string) bool {
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		// A1, A2, A3 = motorways
		if regexp.MustCompile(`\bA\d{1,2}\b`).MatchString(upper) {
			return true
		}
		// DN1, DN2, etc. = national roads (selected)
		if regexp.MustCompile(`\bDN\d{1,3}\b`).MatchString(upper) {
			return true
		}
	}
	return false
}

// Belgium: All motorways (E-roads, A-roads) + selected N-roads
// Viapass network covers ~3,700 km
var beTolledNRoads = map[string]bool{
	"N3": true, "N4": true, "N5": true, "N6": true, "N7": true,
	"N8": true, "N9": true, "N29": true, "N36": true, "N49": true,
	"N54": true, "N58": true, "N60": true, "N62": true, "N63": true,
	"N80": true, "N82": true, "N83": true, "N89": true, "N90": true,
	"N97": true, "N171": true, "N184": true,
}

func matchesBelgium(streetNames []string) bool {
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		refs := roadRef.FindAllString(upper, -1)
		for _, ref := range refs {
			// E-roads and A-roads are always tolled in Belgium
			if (strings.HasPrefix(ref, "E") || strings.HasPrefix(ref, "A")) &&
				len(ref) > 1 && ref[1] >= '0' && ref[1] <= '9' {
				return true
			}
			// Selected N-roads
			if beTolledNRoads[ref] {
				return true
			}
		}
		// R0 (Brussels ring), R1 (Antwerp ring) etc.
		if regexp.MustCompile(`\bR\d{1,2}\b`).MatchString(upper) {
			return true
		}
	}
	return false
}

// Sweden: Only specific bridges/passages have infrastructure charges
var seTollNames = []string{
	"öresundsbron", "oresundsbron", "sundsvallsbron", "motalaström",
}

func matchesSweden(streetNames []string) bool {
	for _, name := range streetNames {
		lower := strings.ToLower(name)
		for _, toll := range seTollNames {
			if strings.Contains(lower, toll) {
				return true
			}
		}
	}
	return false
}

// Denmark: Only Storebælt and Øresund bridges
var dkTollNames = []string{
	"storebælt", "storebaelt", "øresund", "oresund",
}

func matchesDenmark(streetNames []string) bool {
	for _, name := range streetNames {
		lower := strings.ToLower(name)
		for _, toll := range dkTollNames {
			if strings.Contains(lower, toll) {
				return true
			}
		}
	}
	return false
}
