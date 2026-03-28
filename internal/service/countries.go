package service

// countryPolygon holds a simplified border polygon for a single country.
type countryPolygon struct {
	code   string       // ISO 3166-1 alpha-2
	points [][2]float64 // each point is {lat, lon}
}

// countryPolygons lists European countries in priority order.
// Earlier entries win when polygons overlap at simplified borders.
// Small countries are listed first so they are not swallowed by
// their larger neighbours.
var countryPolygons = []countryPolygon{
	// ── Luxembourg (tiny, must be checked first) ─────────────────
	{code: "LU", points: [][2]float64{
		{50.18, 5.74},  // North: Troisvierges
		{50.18, 6.03},  // Clervaux
		{50.05, 6.13},  // Vianden
		{49.87, 6.53},  // Echternach
		{49.68, 6.53},  // Grevenmacher
		{49.55, 6.37},  // SE corner Schengen
		{49.45, 6.11},  // Dudelange
		{49.47, 5.73},  // SW: Pétange
		{49.56, 5.73},  // Arlon junction
		{49.72, 5.74},  // Bastogne area
		{49.91, 5.78},  // Wiltz
	}},

	// ── Netherlands ──────────────────────────────────────────────
	// The Limburg panhandle extends south to ~50.75 at Eijsden;
	// Maastricht sits at 50.85 which is well inside the panhandle.
	{code: "NL", points: [][2]float64{
		{53.47, 5.35},  // Terschelling west
		{53.44, 6.20},  // Ameland
		{53.33, 7.21},  // Emden border area (NE corner)
		{52.63, 7.09},  // Coevorden
		{52.38, 7.07},  // Enschede area
		{52.08, 7.00},  // Winterswijk
		{51.97, 6.69},  // Zevenaar
		{51.87, 6.15},  // Nijmegen area
		{51.75, 5.95},  // Mook
		{51.49, 6.17},  // Venlo
		{51.37, 6.15},  // Roermond
		{51.18, 5.99},  // Sittard
		{50.95, 5.94},  // Geleen area
		{50.84, 5.84},  // East of Maastricht
		{50.75, 5.70},  // Eijsden (southernmost tip)
		{50.75, 5.63},  // Eijsden west bank
		{50.84, 5.62},  // Maastricht west
		{50.95, 5.58},  // Meerssen
		{51.03, 5.55},  // Weert area
		{51.25, 5.45},  // Eindhoven area
		{51.37, 5.02},  // Tilburg
		{51.44, 4.28},  // Breda / Bergen op Zoom
		{51.36, 3.37},  // Zeeuws-Vlaanderen / Terneuzen
		{51.45, 3.60},  // Vlissingen
		{51.59, 3.70},  // Schouwen
		{51.80, 3.85},  // Goeree
		{51.96, 4.05},  // Hook of Holland
		{52.10, 4.25},  // Den Haag coast
		{52.38, 4.53},  // Noordwijk
		{52.63, 4.62},  // Alkmaar coast
		{52.93, 4.78},  // Den Helder
		{53.18, 5.01},  // Afsluitdijk
	}},

	// ── Belgium ──────────────────────────────────────────────────
	{code: "BE", points: [][2]float64{
		{51.48, 2.55},  // De Panne coast
		{51.36, 3.37},  // Zeebrugge / Knokke
		{51.30, 4.24},  // Antwerp area
		{51.25, 5.45},  // Mol / Turnhout
		{51.18, 5.99},  // Maaseik
		{50.76, 6.02},  // Eupen
		{50.51, 6.38},  // Sankt Vith
		{50.17, 6.02},  // Arlon area
		{49.56, 6.37},  // SE corner near Luxembourg
		{49.50, 5.82},  // Virton
		{49.56, 5.47},  // Florenville
		{49.62, 5.12},  // Near Stenay
		{49.84, 4.86},  // Charleville border
		{49.96, 4.68},  // Givet
		{50.10, 4.15},  // Chimay
		{50.10, 3.59},  // Erquelinnes
		{50.33, 3.25},  // Mons
		{50.48, 2.85},  // Tournai
		{50.82, 2.55},  // Menen / Kortrijk
		{51.10, 2.54},  // Ostend area
	}},

	// ── Denmark ──────────────────────────────────────────────────
	// Islands (Zealand, Funen, Lolland) are included in a single
	// simplified polygon that wraps around all major landmasses.
	{code: "DK", points: [][2]float64{
		{54.80, 9.45},  // Padborg / Flensburg border
		{54.85, 8.58},  // Tønder / west coast
		{55.33, 8.13},  // Esbjerg
		{56.10, 8.10},  // Ringkøbing
		{56.83, 8.25},  // Hanstholm
		{57.59, 9.97},  // Skagen
		{57.15, 10.40}, // Frederikshavn
		{56.46, 10.52}, // Grenaa
		{56.18, 12.58}, // Kattegat east (virtual, captures Zealand N)
		{55.80, 12.70}, // Copenhagen north
		{55.55, 12.70}, // Copenhagen south / Amager
		{55.20, 12.80}, // Falster south
		{54.50, 12.00}, // Lolland south
		{54.85, 10.80}, // Langeland
		{55.00, 10.61}, // Svendborg
		{55.23, 9.60},  // Kolding
		{55.03, 9.35},  // Aabenraa
	}},

	// ── Czech Republic (before Poland so Ostrava is matched) ─────
	{code: "CZ", points: [][2]float64{
		{50.87, 14.41}, // Děčín / Elbe exit
		{51.08, 15.04}, // Liberec / Görlitz
		{50.35, 16.34}, // Králíky
		{50.09, 17.07}, // Jeseník
		{49.90, 18.30}, // Ostrava NE (expanded to include Ostrava)
		{49.70, 18.85}, // Třinec
		{49.43, 18.96}, // Jablunkov
		{48.73, 17.50}, // Hodonín
		{48.73, 16.90}, // Břeclav / Mikulov
		{48.73, 14.69}, // Třeboň / Gmünd border
		{48.55, 14.02}, // Český Krumlov
		{48.72, 13.51}, // Šumava
		{49.56, 12.75}, // Cheb area
		{50.27, 12.09}, // Aš / Fichtelgebirge
		{50.63, 12.32}, // Karlovy Vary approach
		{50.80, 13.00}, // Erzgebirge
	}},

	// ── Germany ──────────────────────────────────────────────────
	// Southern border follows the Alps crest; Salzburg (47.80, 13.04)
	// is in Austria, so the border steps north of it.
	{code: "DE", points: [][2]float64{
		{54.85, 8.58},  // Sylt
		{54.80, 9.45},  // Flensburg
		{54.40, 10.19}, // Kiel
		{54.18, 12.10}, // Rostock
		{54.09, 13.38}, // Stralsund
		{53.92, 14.22}, // Usedom
		{53.12, 14.44}, // Schwedt / Oder
		{52.35, 14.55}, // Frankfurt/Oder
		{51.67, 14.98}, // Cottbus
		{51.08, 15.04}, // Görlitz
		{50.87, 14.41}, // Saxon Switzerland
		{50.63, 12.32}, // Plauen
		{50.27, 12.09}, // Cheb border
		{49.56, 12.75}, // Passau approach
		{48.77, 13.83}, // Passau
		{48.00, 12.90}, // Chiemsee area (N of Salzburg)
		{47.60, 12.15}, // Reit im Winkl
		{47.45, 11.35}, // Garmisch-Partenkirchen
		{47.53, 9.68},  // Lindau
		{47.71, 8.68},  // Konstanz area
		{47.81, 7.63},  // Basel corner (DE side)
		{48.97, 8.23},  // Karlsruhe
		{49.21, 6.84},  // Saarbrücken
		{49.47, 6.36},  // Trier
		{50.05, 6.13},  // Prüm
		{50.76, 6.02},  // Aachen
		{51.97, 6.69},  // Lower Rhine
		{52.38, 7.07},  // Bentheim
		{53.33, 7.21},  // Emden
		{53.60, 8.13},  // Wilhelmshaven
		{53.87, 8.70},  // Cuxhaven
		{54.30, 8.58},  // Husum
	}},

	// ── France ───────────────────────────────────────────────────
	{code: "FR", points: [][2]float64{
		{51.08, 2.54},  // Dunkirk
		{50.82, 1.58},  // Calais
		{49.48, 0.12},  // Le Havre
		{48.64, -1.17}, // Saint-Malo
		{48.45, -4.50}, // Brest
		{47.65, -3.02}, // Lorient
		{47.28, -2.19}, // Saint-Nazaire
		{46.16, -1.15}, // La Rochelle
		{45.60, -1.22}, // Royan
		{44.65, -1.17}, // Arcachon
		{43.40, -1.79}, // Bayonne / Biarritz
		{42.67, -1.79}, // Pyrenees W
		{42.43, 0.77},  // Pyrenees central
		{42.33, 3.18},  // Perpignan
		{43.25, 3.47},  // Narbonne coast
		{43.10, 5.95},  // Toulon
		{43.70, 7.27},  // Nice / Monaco
		{44.10, 7.66},  // Col de Tende
		{45.30, 6.87},  // Mont Cenis
		{45.83, 7.04},  // Mont Blanc
		{46.21, 6.12},  // Geneva area
		{46.87, 6.85},  // Pontarlier
		{47.41, 7.55},  // Belfort
		{47.81, 7.51},  // Mulhouse / Basel approach
		{48.97, 8.23},  // Strasbourg area
		{49.21, 6.84},  // Saargemünd
		{49.47, 6.36},  // France-Luxembourg border
		{49.56, 5.47},  // Longwy
		{49.96, 4.68},  // Givet
		{50.10, 3.59},  // Maubeuge
		{50.48, 2.85},  // Valenciennes
	}},

	// ── Switzerland ──────────────────────────────────────────────
	{code: "CH", points: [][2]float64{
		{47.81, 7.63},  // Basel
		{47.71, 8.68},  // Schaffhausen area
		{47.53, 9.68},  // St. Margrethen / Bodensee E
		{47.05, 9.60},  // Liechtenstein border
		{46.86, 10.49}, // Engadin E (Martina)
		{46.49, 10.48}, // Val Müstair
		{46.19, 10.15}, // Stelvio approach
		{46.00, 9.28},  // Chiavenna area
		{45.82, 8.96},  // Lugano
		{45.85, 8.59},  // Ponte Tresa
		{46.15, 7.85},  // Simplon approach
		{45.92, 7.04},  // Great St Bernard
		{46.21, 6.12},  // Geneva SW
		{46.43, 6.06},  // Nyon
		{46.87, 6.85},  // Jura / Pontarlier border
		{47.41, 7.55},  // Delémont / Belfort gap
	}},

	// ── Austria ──────────────────────────────────────────────────
	{code: "AT", points: [][2]float64{
		{48.78, 16.90}, // NE corner (Bratislava approach)
		{48.01, 17.16}, // Neusiedl / Sopron area
		{47.40, 16.54}, // Szombathely border
		{46.87, 16.11}, // Jennersdorf
		{46.62, 15.64}, // Spielfeld
		{46.52, 14.55}, // Karawanken
		{46.65, 13.71}, // Villach
		{46.66, 12.44}, // Lienz
		{47.00, 11.49}, // Brenner
		{47.27, 11.35}, // Innsbruck
		{47.27, 10.18}, // Arlberg
		{47.53, 9.68},  // Bregenz / Bodensee
		{47.05, 9.60},  // Feldkirch
		{47.27, 10.18}, // Arlberg (return)
		{47.45, 11.35}, // Garmisch border
		{47.60, 12.15}, // Reit im Winkl
		{48.00, 12.90}, // N of Salzburg
		{48.15, 12.76}, // Braunau
		{48.77, 13.83}, // Passau corner
		{48.73, 14.69}, // Freistadt / Summerau
		{48.87, 15.03}, // Gmünd
		{48.77, 15.76}, // Horn area
		{48.69, 16.45}, // Hollabrunn
	}},

	// ── Italy ────────────────────────────────────────────────────
	{code: "IT", points: [][2]float64{
		{43.70, 7.27},  // Ventimiglia / Nice border
		{44.10, 7.66},  // Col de Tende
		{45.30, 6.87},  // Mont Cenis
		{45.83, 7.04},  // Mont Blanc
		{45.92, 7.04},  // Great St Bernard
		{46.15, 7.85},  // Simplon
		{45.85, 8.59},  // Ponte Tresa
		{45.82, 8.96},  // Lugano area
		{46.00, 9.28},  // Chiavenna
		{46.19, 10.15}, // Stelvio
		{46.49, 10.48}, // Val Müstair
		{47.00, 11.49}, // Brenner
		{46.66, 12.44}, // Lienz / Sillian
		{46.65, 13.71}, // Tarvisio
		{46.52, 14.55}, // Karawanken S
		{45.63, 13.78}, // Trieste
		{44.05, 13.56}, // Pula (approximate Adriatic)
		{42.04, 15.35}, // Gargano
		{40.64, 18.00}, // Lecce area
		{39.87, 18.51}, // Otranto
		{37.95, 16.07}, // Calabria SE
		{37.93, 15.65}, // Reggio Calabria
		{38.88, 16.01}, // Pizzo
		{39.00, 15.63}, // Cosenza W
		{40.05, 15.34}, // Cilento
		{40.60, 14.36}, // Amalfi
		{41.20, 13.55}, // Gaeta
		{42.10, 11.12}, // Orbetello
		{43.57, 10.30}, // Livorno
		{44.10, 9.68},  // La Spezia
		{44.30, 8.24},  // Savona
	}},

	// ── Spain ────────────────────────────────────────────────────
	{code: "ES", points: [][2]float64{
		{43.40, -1.79}, // Basque coast
		{43.36, -3.02}, // Santander
		{43.47, -5.88}, // Gijón
		{43.37, -8.40}, // A Coruña
		{42.88, -9.27}, // Finisterre
		{42.10, -8.90}, // Vigo
		{41.87, -8.87}, // Portuguese border N
		{41.70, -7.18}, // Bragança
		{41.10, -6.93}, // Portuguese border central
		{39.46, -7.53}, // Badajoz area
		{37.95, -7.40}, // Huelva border
		{36.00, -5.60}, // Gibraltar area
		{36.72, -2.17}, // Almería
		{37.60, -0.75}, // Murcia coast
		{38.75, -0.04}, // Valencia area
		{40.56, 0.52},  // Tarragona coast
		{41.86, 3.12},  // Cap de Creus
		{42.33, 3.18},  // Border with France
		{42.43, 0.77},  // Pyrenees central
		{42.67, -1.79}, // Pyrenees W
	}},

	// ── Poland ───────────────────────────────────────────────────
	{code: "PL", points: [][2]float64{
		{54.35, 18.64}, // Gdańsk
		{54.83, 18.38}, // Hel peninsula
		{54.79, 17.53}, // Słupsk coast
		{54.45, 16.87}, // Koszalin coast
		{53.92, 14.22}, // Szczecin / Świnoujście
		{53.12, 14.44}, // Oder river
		{52.35, 14.55}, // Frankfurt/Oder
		{51.67, 14.98}, // Forst
		{51.08, 15.04}, // Zgorzelec / Görlitz
		{50.35, 16.34}, // Kłodzko
		{50.09, 17.07}, // Opole area
		{49.90, 18.30}, // Cieszyn area
		{49.70, 18.85}, // Třinec border
		{49.43, 18.96}, // Żilina border
		{49.30, 20.07}, // Tatras
		{49.32, 22.37}, // Bieszczady
		{50.09, 24.00}, // Przemyśl area
		{51.24, 23.62}, // Bug river
		{52.08, 23.80}, // Brest area
		{53.50, 23.90}, // Suwałki gap approach
		{54.35, 22.78}, // Kaliningrad border
		{54.38, 19.46}, // Elbląg / Vistula Lagoon
	}},
}

// countryFromCoord returns the ISO 3166-1 alpha-2 country code for the
// given latitude/longitude, or an empty string when the point does not
// fall inside any of the known polygons.
// Polygons are tested in priority order so that small countries
// (LU, NL, BE) are matched before their larger neighbours.
func countryFromCoord(lat, lon float64) string {
	for _, cp := range countryPolygons {
		if pointInPolygon(lat, lon, cp.points) {
			return cp.code
		}
	}
	return ""
}

// pointInPolygon uses the ray-casting algorithm to decide whether
// the point (lat, lon) is inside the polygon defined by pts.
// The polygon is implicitly closed (last vertex connects to first).
func pointInPolygon(lat, lon float64, pts [][2]float64) bool {
	n := len(pts)
	if n < 3 {
		return false
	}

	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		yi, xi := pts[i][0], pts[i][1]
		yj, xj := pts[j][0], pts[j][1]

		// Does the edge from j→i straddle the horizontal ray from (lat, lon) going right?
		if (yi > lat) != (yj > lat) {
			// Compute the x-intersection of the edge with the horizontal line y = lat.
			xIntersect := xi + (lat-yi)/(yj-yi)*(xj-xi)
			if lon < xIntersect {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}
