package data

// TollRoadsEU contains the official list of tolled road numbers per country.
// These are based on official sources from each country's toll operator.
// Last updated: March 2026.
//
// Sources:
//   DE: BALM Mauttabelle — balm.bund.de (all Bundesfernstraßen, ~51,000 km)
//   NL: vrachtwagenheffing.nl — "Lijst met wegen" (effective 2026)
//   BE: Viapass — viapass.be (motorways + selected N-roads, ~6,800 km)
//   FR: ASFA / WikiSara — toll concession autoroutes (excl. free A-roads)
//   AT: ASFINAG — asfinag.at (Autobahnen + Schnellstraßen, ~2,250 km)
//   CH: LSVA — all roads (100%)
//   IT: AISCAT — autostrade.it (~6,000 km)
//   ES: DGT — autopistas de peaje
//   PL: e-TOLL — etoll.gov.pl (~5,870 km)
//   CZ: MYTO CZ — mytocz.eu (dálnice + selected class I roads)
//   HU: HU-GO — hu-go.hu
//   SK: Emyto — emyto.sk
//   SI: DarsGo — dfrpo.si
//   PT: Via Verde — infraestruturasdeportugal.pt
//   BG: Agency Road Infrastructure — api.bg
//   RO: CNAIR — cfrr.ro
//   HR: HAC/ARZ — hac.hr
//   SE: Transportstyrelsen — specific bridges/passages only
//   DK: Vejdirektoratet — specific bridges only

// ── Germany ─────────────────────────────────────────────────────────
// ALL Bundesautobahnen and ALL Bundesstraßen are tolled for vehicles >3.5t.
// Since July 2024, the entire 51,000 km Bundesfernstraßennetz is tolled.
// Exceptions: A6 (FR border–Saarbrücken-Fechingen), A5 (CH/FR border–Müllheim/Neuenburg)
// Matching strategy: prefix-based (A* + B*) because ALL are tolled.
// The two exceptions are negligible segments.
var DE_TollPrefixes = []string{"A", "B"}

// DE roads explicitly EXCLUDED from toll (BFStrMG exceptions)
var DE_TollExceptions = map[string]bool{
	// A6: deutsch-französische Grenze bis AS Saarbrücken-Fechingen
	// A5: deutsch-schweizerische/französische Grenze bis AS Müllheim/Neuenburg
	// These are very short segments; we accept the minor inaccuracy.
}

// ── Austria ─────────────────────────────────────────────────────────
// Source: ASFINAG / Bundesstraßengesetz — all Autobahnen (A) and Schnellstraßen (S)
var AT_TollRoads = map[string]bool{
	// Autobahnen
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true,
	"A6": true, "A7": true, "A8": true, "A9": true, "A10": true,
	"A11": true, "A12": true, "A13": true, "A14": true,
	"A21": true, "A22": true, "A23": true, "A25": true, "A26": true,
	// Schnellstraßen
	"S1": true, "S2": true, "S3": true, "S4": true, "S5": true,
	"S6": true, "S10": true, "S16": true, "S31": true, "S33": true,
	"S35": true, "S36": true, "S37": true,
}

// ── France ──────────────────────────────────────────────────────────
// France has a mix of tolled (concession) and free (state) autoroutes.
// ~75% of autoroutes are tolled, ~25% are free.
// Source: ASFA, WikiSara "Liste des sections d'autoroutes françaises gratuites"
//
// Strategy: list FREE autoroutes explicitly, treat all others as tolled.
// This is more accurate because the free list is shorter and well-documented.
var FR_FreeAutoroutes = map[string]bool{
	// Completely free A-roads
	"A75":  true, // La Méridienne (Clermont-Ferrand–Béziers), except Viaduc de Millau
	"A84":  true, // Normandy–Brittany (Rennes–Caen)
	"A35":  true, // Alsace (Strasbourg–Mulhouse)
	"A36":  true, // Beaune–Mulhouse (partially free)
	"A630": true, // Bordeaux ring
	"A620": true, // Toulouse ring
	"A621": true, // Toulouse–Muret
	"A624": true, // Toulouse area
	"A660": true, // Arcachon
	"A680": true, // Toulouse area

	// Brittany — ALL autoroutes are free
	"A82":  true, // Nantes area
	"A84b": true, // Brittany
	"A11b": true, // section in Brittany

	// Northern France free sections
	"A16":  true, // Boulogne–Belgian border (Calais/Dunkerque section)
	"A25":  true, // Dunkerque–Lille
	"A34":  true, // Reims–Belgian border
	"A26b": true, // Some free sections
	"A28b": true, // Free sections around Rouen

	// Provence / urban free sections
	"A55":  true, // Marseille–Martigues
	"A507": true, // Marseille L2 bypass
	"A570": true, // Toulon–Hyères
	"A500": true, // Monaco area
	"A520": true, // Aix-en-Provence area
}

// France: roads that are ALWAYS tolled (major concession autoroutes)
// If a road is not in FR_FreeAutoroutes and starts with A, it's tolled.

// ── Belgium ─────────────────────────────────────────────────────────
// Source: Viapass — all motorways + ring roads + selected N-roads
// The E-road numbering is the primary reference in Belgium
var BE_TollMotorways = map[string]bool{
	// E-roads (all tolled in Belgium for trucks >3.5t)
	"E17": true, "E19": true, "E25": true, "E34": true, "E40": true,
	"E42": true, "E403": true, "E411": true, "E313": true, "E314": true,

	// A-roads (Belgian motorway numbering, all tolled)
	"A1": true, "A2": true, "A3": true, "A4": true, "A7": true,
	"A8": true, "A10": true, "A12": true, "A13": true, "A14": true,
	"A15": true, "A17": true, "A19": true, "A21": true, "A25": true,
	"A26": true, "A27": true, "A28": true, "A54": true, "A501": true,
	"A503": true, "A601": true, "A602": true, "A604": true,

	// Ring roads
	"R0": true, "R1": true, "R2": true, "R3": true, "R4": true,
	"R5": true, "R6": true, "R8": true, "R9": true,
}

// Belgium: selected N-roads in the Viapass network
var BE_TollNRoads = map[string]bool{
	"N3": true, "N4": true, "N5": true, "N6": true, "N7": true,
	"N8": true, "N9": true, "N20": true, "N25": true, "N29": true,
	"N36": true, "N49": true, "N54": true, "N58": true, "N60": true,
	"N62": true, "N63": true, "N80": true, "N82": true, "N83": true,
	"N89": true, "N90": true, "N97": true, "N171": true, "N184": true,
	"N257": true, "N271": true, "N280": true, "N320": true, "N329": true,
	"N350": true, "N368": true, "N382": true, "N391": true, "N602": true,
}

// ── Italy ───────────────────────────────────────────────────────────
// Source: AISCAT / Autostrade per l'Italia
// Most A-roads are tolled, but some sections are free (urban bypasses, Calabria)
var IT_TollRoads = map[string]bool{
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true,
	"A6": true, "A7": true, "A8": true, "A9": true, "A10": true,
	"A11": true, "A12": true, "A13": true, "A14": true, "A15": true,
	"A16": true, "A18": true, "A20": true, "A21": true, "A22": true,
	"A23": true, "A24": true, "A25": true, "A26": true, "A27": true,
	"A28": true, "A29": true, "A30": true, "A31": true, "A32": true,
	"A33": true, "A34": true, "A35": true,
}

// Italy: free A-road sections (tangenziali / urban bypasses)
var IT_FreeARoads = map[string]bool{
	// Urban tangenziali are technically A-numbered but toll-free
	// These are minor; we accept the inaccuracy for now
}

// ── Spain ───────────────────────────────────────────────────────────
// Source: DGT — only autopistas de peaje (AP) are tolled, autovías (A) are free
var ES_TollRoads = map[string]bool{
	"AP1": true, "AP2": true, "AP4": true, "AP6": true, "AP7": true,
	"AP8": true, "AP9": true, "AP15": true, "AP36": true, "AP41": true,
	"AP46": true, "AP51": true, "AP53": true, "AP61": true, "AP66": true,
	"AP68": true, "AP71": true,
	// Catalan C-roads with toll
	"C31": true, "C32": true, "C33": true,
}

// ── Poland ──────────────────────────────────────────────────────────
// Source: e-TOLL / GDDKiA — etoll.gov.pl
// For vehicles >3.5t: all autostrady (A), all drogi ekspresowe (S),
// and selected drogi krajowe (DK)
var PL_TollAutostrady = map[string]bool{
	"A1": true, "A2": true, "A4": true, "A6": true, "A8": true, "A18": true,
}

var PL_TollEkspresowe = map[string]bool{
	"S1": true, "S2": true, "S3": true, "S5": true, "S6": true, "S7": true,
	"S8": true, "S10": true, "S11": true, "S12": true, "S14": true,
	"S16": true, "S17": true, "S19": true, "S22": true, "S51": true,
	"S52": true, "S61": true, "S69": true, "S74": true, "S79": true,
	"S86": true,
}

// Selected national roads (drogi krajowe) in e-TOLL for >3.5t
var PL_TollDrogiKrajowe = map[string]bool{
	"DK1": true, "DK2": true, "DK3": true, "DK4": true, "DK5": true,
	"DK6": true, "DK7": true, "DK8": true, "DK9": true, "DK10": true,
	"DK11": true, "DK12": true, "DK14": true, "DK15": true, "DK16": true,
	"DK17": true, "DK18": true, "DK19": true, "DK20": true, "DK22": true,
	"DK25": true, "DK28": true, "DK32": true, "DK36": true, "DK44": true,
	"DK46": true, "DK50": true, "DK61": true, "DK63": true, "DK73": true,
	"DK74": true, "DK77": true, "DK78": true, "DK79": true, "DK81": true,
	"DK86": true, "DK88": true, "DK91": true, "DK92": true, "DK94": true,
}

// ── Czech Republic ──────────────────────────────────────────────────
// Source: MYTO CZ — mytocz.eu
// Dálnice (motorways) + selected silnice I. třídy (class I roads)
var CZ_TollDalnice = map[string]bool{
	"D0": true, "D1": true, "D2": true, "D3": true, "D4": true,
	"D5": true, "D6": true, "D7": true, "D8": true, "D10": true,
	"D11": true, "D35": true, "D46": true, "D48": true, "D52": true,
	"D55": true, "D56": true,
}

// CZ class I roads in MYTO system
var CZ_TollClassI = map[string]bool{
	"I/3": true, "I/6": true, "I/7": true, "I/8": true, "I/9": true,
	"I/11": true, "I/13": true, "I/14": true, "I/16": true, "I/20": true,
	"I/21": true, "I/23": true, "I/25": true, "I/27": true, "I/33": true,
	"I/34": true, "I/35": true, "I/36": true, "I/37": true, "I/38": true,
	"I/43": true, "I/44": true, "I/46": true, "I/48": true, "I/49": true,
	"I/50": true, "I/52": true, "I/53": true, "I/55": true, "I/56": true,
	"I/57": true, "I/58": true, "I/67": true, "I/68": true, "I/69": true,
}

// ── Hungary ─────────────────────────────────────────────────────────
// Source: HU-GO — hu-go.hu
var HU_TollRoads = map[string]bool{
	// Motorways (M)
	"M0": true, "M1": true, "M2": true, "M3": true, "M4": true,
	"M5": true, "M6": true, "M7": true, "M8": true, "M9": true,
	"M15": true, "M19": true, "M25": true, "M30": true, "M31": true,
	"M35": true, "M43": true, "M44": true, "M60": true, "M70": true,
	"M85": true, "M86": true,
	// Selected main roads (főutak)
	// HU-GO includes selected sections of routes 1-8, 10, 21, 25, 35, etc.
	// For now we include only M-roads; expand as official data becomes available
}

// ── Slovakia ────────────────────────────────────────────────────────
// Source: Emyto — emyto.sk
var SK_TollRoads = map[string]bool{
	// Diaľnice
	"D1": true, "D2": true, "D3": true, "D4": true,
	// Rýchlostné cesty
	"R1": true, "R2": true, "R3": true, "R4": true, "R5": true, "R6": true, "R7": true,
	// Selected Class I roads (cesty I. triedy)
	"I/2": true, "I/9": true, "I/11": true, "I/18": true, "I/50": true,
	"I/51": true, "I/59": true, "I/61": true, "I/63": true, "I/65": true,
	"I/66": true, "I/68": true, "I/75": true,
}

// ── Slovenia ────────────────────────────────────────────────────────
// Source: DarsGo — dfrpo.si / DARS
var SI_TollRoads = map[string]bool{
	// Avtoceste
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true,
	// Hitre ceste
	"H2": true, "H3": true, "H4": true, "H5": true, "H6": true, "H7": true,
}

// ── Portugal ────────────────────────────────────────────────────────
// Source: Infraestruturas de Portugal / Via Verde
var PT_TollRoads = map[string]bool{
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true,
	"A6": true, "A7": true, "A8": true, "A9": true, "A10": true,
	"A11": true, "A12": true, "A13": true, "A14": true, "A15": true,
	"A16": true, "A17": true, "A22": true, "A23": true, "A24": true,
	"A25": true, "A27": true, "A28": true, "A29": true, "A32": true,
	"A33": true, "A41": true, "A42": true, "A43": true, "A44": true,
}

// ── Bulgaria ────────────────────────────────────────────────────────
// Source: Agency Road Infrastructure — api.bg
// Toll for >3.5t on motorways and selected class I roads
var BG_TollRoads = map[string]bool{
	// Avtomagistrali
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true, "A6": true,
	// Selected class I roads (I-numbered)
	"I1": true, "I2": true, "I3": true, "I4": true, "I5": true, "I6": true,
	"I8": true, "I9": true,
}

// ── Romania ─────────────────────────────────────────────────────────
// Source: CNAIR — cfrr.ro
var RO_TollAutostrazi = map[string]bool{
	"A0": true, "A1": true, "A2": true, "A3": true, "A4": true,
	"A7": true, "A10": true, "A11": true,
}

// RO: selected drumuri naționale (DN)
// The RO-vignette system covers all national roads for <3.5t
// For >3.5t, the MYTO system charges on motorways + selected DN
var RO_TollDN = map[string]bool{
	"DN1": true, "DN1A": true, "DN2": true, "DN2A": true, "DN3": true,
	"DN5": true, "DN6": true, "DN7": true, "DN13": true, "DN14": true,
	"DN15": true, "DN17": true, "DN28": true, "DN39": true, "DN65": true,
	"DN66": true, "DN67": true, "DN68": true, "DN69": true, "DN73": true,
}

// ── Croatia ─────────────────────────────────────────────────────────
// Source: HAC (hac.hr) / ARZ (arz.hr)
var HR_TollRoads = map[string]bool{
	"A1": true, "A2": true, "A3": true, "A4": true, "A5": true,
	"A6": true, "A7": true, "A8": true, "A9": true, "A10": true,
	"A11": true,
}

// ── Sweden ──────────────────────────────────────────────────────────
// Sweden has NO general truck toll. Only congestion taxes and bridge tolls.
var SE_TollBridges = []string{
	"öresundsbron", "oresundsbron", "øresund",
	"svinesundsbron", "svinesund",
	"sundsvallsbron", "sundsvall",
	"motalaström", "motala",
}

// Stockholm and Gothenburg congestion tax zones (area-based, not road-specific)
// These are harder to match from road names; we skip for now.

// ── Denmark ─────────────────────────────────────────────────────────
// Denmark has NO general truck toll. Only bridge tolls.
var DK_TollBridges = []string{
	"storebælt", "storebaelt", "great belt",
	"øresund", "oresund", "öresund",
}

// ── Switzerland ─────────────────────────────────────────────────────
// LSVA (Leistungsabhängige Schwerverkehrsabgabe) applies to ALL roads.
// No specific list needed — IsTollRoad always returns true for CH.
