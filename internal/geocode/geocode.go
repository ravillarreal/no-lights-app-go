// Package geocode reverse-geocodes coordinates via OSM Nominatim (free tier,
// 1 req/sec max).
package geocode

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Address struct {
	Neighborhood string
	Municipality string
	State        string
	Country      string
}

type nominatimResponse struct {
	Address map[string]string `json:"address"`
}

// Lookup resolves (lon, lat) to a structured address. It enforces the
// Nominatim 1 req/sec rate limit by sleeping before the call.
func Lookup(ctx context.Context, lon, lat float64) (Address, string, error) {
	// Nominatim asks for max 1 req/sec — sleep before each call.
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return Address{}, "", ctx.Err()
	}

	u := "https://nominatim.openstreetmap.org/reverse?" + url.Values{
		"lat":            {strconv.FormatFloat(lat, 'f', -1, 64)},
		"lon":            {strconv.FormatFloat(lon, 'f', -1, 64)},
		"format":         {"json"},
		"addressdetails": {"1"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Address{}, "", err
	}
	req.Header.Set("User-Agent", "no-lights-app-go/1.0 (rafaelvillarreal2000@gmail.com)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Address{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Nominatim returned %d", resp.StatusCode)
		return Address{}, "", nil
	}

	var parsed nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Address{}, "", err
	}
	a := parsed.Address
	raw, _ := json.Marshal(a)
	return Address{
		Neighborhood: firstNonEmpty(a["neighbourhood"], a["suburb"], a["quarter"], a["city_district"], a["hamlet"], a["isolated_dwelling"]),
		Municipality: firstNonEmpty(a["city"], a["town"], a["village"], a["municipality"], a["county"], a["district"]),
		State:        a["state"],
		Country:      a["country"],
	}, string(raw), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
