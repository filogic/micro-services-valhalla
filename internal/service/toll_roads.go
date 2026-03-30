package service

import (
	"regexp"
	"strings"

	"github.com/filogic/micro-services-valhalla/internal/data"
)

// IsTollRoad checks if the given street names indicate a tolled road
// in the specified country. Uses official road lists per country.
func IsTollRoad(country string, streetNames []string) bool {
	switch country {
	case "NL":
		return IsNLTollRoad(streetNames)
	case "CH":
		return true // LSVA: all roads
	case "DE":
		return matchesDEPrefixes(streetNames)
	case "FR":
		return matchesFrance(streetNames)
	case "AT":
		return matchesExactList(streetNames, data.AT_TollRoads)
	case "BE":
		return matchesBelgium(streetNames)
	case "IT":
		return matchesExactList(streetNames, data.IT_TollRoads)
	case "ES":
		return matchesExactList(streetNames, data.ES_TollRoads)
	case "PL":
		return matchesPoland(streetNames)
	case "CZ":
		return matchesCzechia(streetNames)
	case "HU":
		return matchesExactList(streetNames, data.HU_TollRoads)
	case "SK":
		return matchesExactList(streetNames, data.SK_TollRoads)
	case "SI":
		return matchesExactList(streetNames, data.SI_TollRoads)
	case "PT":
		return matchesExactList(streetNames, data.PT_TollRoads)
	case "BG":
		return matchesExactList(streetNames, data.BG_TollRoads)
	case "RO":
		return matchesRomania(streetNames)
	case "HR":
		return matchesExactList(streetNames, data.HR_TollRoads)
	case "SE":
		return matchesBridgeNames(streetNames, data.SE_TollBridges)
	case "DK":
		return matchesBridgeNames(streetNames, data.DK_TollBridges)
	default:
		return false
	}
}

// ── Helpers ─────────────────────────────────────────────────────────

// roadRefPattern extracts road references like "A1", "B42", "D1", "S7", "AP7"
var roadRefPattern = regexp.MustCompile(`(?i)\b([A-Z]{1,2}\d{1,4}[A-Z]?)\b`)

// extractRoadRefs finds all road reference numbers from street names.
func extractRoadRefs(streetNames []string) []string {
	var refs []string
	for _, name := range streetNames {
		matches := roadRefPattern.FindAllString(strings.ToUpper(name), -1)
		refs = append(refs, matches...)
	}
	return refs
}

// matchesExactList checks if any road ref appears in the given toll road set.
func matchesExactList(streetNames []string, tollRoads map[string]bool) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if tollRoads[ref] {
			return true
		}
	}
	return false
}

// matchesBridgeNames checks if any street name contains a known bridge/tunnel name.
func matchesBridgeNames(streetNames []string, tollNames []string) bool {
	for _, name := range streetNames {
		lower := strings.ToLower(name)
		for _, toll := range tollNames {
			if strings.Contains(lower, toll) {
				return true
			}
		}
	}
	return false
}

// ── Country-specific matchers ───────────────────────────────────────

// Germany: ALL Autobahnen (A*) and ALL Bundesstraßen (B*) are tolled.
// This is prefix-based because the official network is the entire
// Bundesfernstraßennetz (~51,000 km). Source: BALM / BFStrMG.
func matchesDEPrefixes(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		for _, prefix := range data.DE_TollPrefixes {
			if strings.HasPrefix(ref, prefix) && len(ref) > len(prefix) {
				rest := ref[len(prefix):]
				if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
					return true
				}
			}
		}
	}
	return false
}

// France: most A-roads are tolled EXCEPT those in FR_FreeAutoroutes.
// ~75% tolled, ~25% free. The free list includes Brittany, A75, A20, A35, etc.
func matchesFrance(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if strings.HasPrefix(ref, "A") && len(ref) > 1 && ref[1] >= '0' && ref[1] <= '9' {
			// It's an A-road — check if it's in the FREE list
			if !data.FR_FreeAutoroutes[ref] {
				return true // tolled (not in free list)
			}
		}
	}
	return false
}

// Belgium: motorways (E, A, R roads) + selected N-roads.
func matchesBelgium(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if data.BE_TollMotorways[ref] || data.BE_TollNRoads[ref] {
			return true
		}
	}
	// Also check E-road format in full street names (e.g., "E40/A10")
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		matches := regexp.MustCompile(`\bE\d{1,3}\b`).FindAllString(upper, -1)
		for _, m := range matches {
			if data.BE_TollMotorways[m] {
				return true
			}
		}
	}
	return false
}

// Poland: autostrady (A) + ekspresowe (S) + selected drogi krajowe (DK)
func matchesPoland(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if data.PL_TollAutostrady[ref] || data.PL_TollEkspresowe[ref] || data.PL_TollDrogiKrajowe[ref] {
			return true
		}
	}
	// Also check for DK format: "DK1", "DK94" etc.
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		matches := regexp.MustCompile(`\bDK\d{1,3}\b`).FindAllString(upper, -1)
		for _, m := range matches {
			if data.PL_TollDrogiKrajowe[m] {
				return true
			}
		}
	}
	return false
}

// Czechia: dálnice (D) + selected class I roads (I/xx format)
func matchesCzechia(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if data.CZ_TollDalnice[ref] {
			return true
		}
	}
	// Check for class I road format: "I/35", "I/6" etc.
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		matches := regexp.MustCompile(`\bI/\d{1,2}\b`).FindAllString(upper, -1)
		for _, m := range matches {
			if data.CZ_TollClassI[m] {
				return true
			}
		}
	}
	return false
}

// Romania: autostrăzi (A) + selected drumuri naționale (DN)
func matchesRomania(streetNames []string) bool {
	for _, ref := range extractRoadRefs(streetNames) {
		if data.RO_TollAutostrazi[ref] {
			return true
		}
	}
	// Check DN format
	for _, name := range streetNames {
		upper := strings.ToUpper(name)
		matches := regexp.MustCompile(`\bDN\d{1,3}[A-Z]?\b`).FindAllString(upper, -1)
		for _, m := range matches {
			if data.RO_TollDN[m] {
				return true
			}
		}
	}
	return false
}
