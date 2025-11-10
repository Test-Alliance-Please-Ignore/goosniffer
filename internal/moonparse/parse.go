package moonparse

import (
	"regexp"
	"strings"
)

// Outer key: moon name  (e.g. "66-PMM V - Moon 15")
// Inner key: product    (e.g. "Flawless Arkonor")
// Value: data about that product on that moon.
type MoonProductData struct {
	Quantity      string `json:"quantity"`
	OreTypeID     string `json:"ore_type_id"`
	SolarSystemID string `json:"solar_system_id"`
	PlanetID      string `json:"planet_id"`
	MoonID        string `json:"moon_id"`
}

type MoonProducts map[string]map[string]MoonProductData

// Example moon line:
//
//	66-PMM V - Moon 15
//
// Capture whole "66-PMM V - Moon 15" as group 1.
var moonLineRe = regexp.MustCompile(`^\s*(.+ - Moon \d+)\s*$`)

// Example product line:
//
//	Flawless Arkonor    0.323762148619    46678    30004923    40311969    40311985
//
// Groups:
//
//	1: product name
//	2: quantity
//	3: ore typeID
//	4: solarSystemID
//	5: planetID
//	6: moonID
var productLineRe = regexp.MustCompile(
	`^\s*(.+?)\s+([0-9]+(?:\.[0-9]+)?)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s*$`,
)

func ParseMoons(input string) (MoonProducts, error) {
	result := make(MoonProducts)
	var currentMoon string

	lines := strings.Split(input, "\n")
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip header line
		if strings.Contains(trimmed, "Moon Product") {
			continue
		}

		// Detect moon name
		if m := moonLineRe.FindStringSubmatch(line); len(m) == 2 {
			currentMoon = m[1]
			if _, ok := result[currentMoon]; !ok {
				result[currentMoon] = make(map[string]MoonProductData)
			}
			continue
		}
		if currentMoon == "" {
			continue
		}

		// Detect product rows
		if m := productLineRe.FindStringSubmatch(line); len(m) == 7 {
			product := m[1]
			result[currentMoon][product] = MoonProductData{
				Quantity:      m[2],
				OreTypeID:     m[3],
				SolarSystemID: m[4],
				PlanetID:      m[5],
				MoonID:        m[6],
			}
		}
	}
	return result, nil
}
