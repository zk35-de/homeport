package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WeatherData holds current weather and a short forecast.
type WeatherData struct {
	CityName    string
	Temperature float64
	WeatherCode int
	Description string
	WindSpeed   float64
	Humidity    int
	IsDay       bool
	Forecast    []DayForecast
}

// DayForecast holds the forecast for one day.
type DayForecast struct {
	Date    string
	TempMax float64
	TempMin float64
	Code    int
	Desc    string
}

// openMeteoResponse mirrors the Open-Meteo JSON response structure.
type openMeteoResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		WeatherCode int     `json:"weather_code"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		IsDay       int     `json:"is_day"`
		Humidity    int     `json:"relative_humidity_2m"`
	} `json:"current"`
	Daily struct {
		Time        []string  `json:"time"`
		WeatherCode []int     `json:"weather_code"`
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

// FetchWeather retrieves current weather and a 5-day forecast from Open-Meteo.
func FetchWeather(lat, lon float64) (*WeatherData, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
			"&current=temperature_2m,weather_code,wind_speed_10m,is_day,relative_humidity_2m"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min"+
			"&timezone=auto&forecast_days=5",
		lat, lon,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("weather fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %s", resp.Status)
	}

	var raw openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("weather decode failed: %w", err)
	}

	wd := &WeatherData{
		Temperature: raw.Current.Temperature,
		WeatherCode: raw.Current.WeatherCode,
		Description: weatherDescription(raw.Current.WeatherCode),
		WindSpeed:   raw.Current.WindSpeed,
		Humidity:    raw.Current.Humidity,
		IsDay:       raw.Current.IsDay == 1,
	}

	// Build forecast (skip today = index 0)
	for i := 1; i < len(raw.Daily.Time) && i < len(raw.Daily.WeatherCode); i++ {
		t, err := time.Parse("2006-01-02", raw.Daily.Time[i])
		if err != nil {
			continue
		}
		label := t.Format("02.01") // e.g. "21.03"
		weekday := germanWeekday(t.Weekday())
		var tMax, tMin float64
		if i < len(raw.Daily.TempMax) {
			tMax = raw.Daily.TempMax[i]
		}
		if i < len(raw.Daily.TempMin) {
			tMin = raw.Daily.TempMin[i]
		}
		code := raw.Daily.WeatherCode[i]
		wd.Forecast = append(wd.Forecast, DayForecast{
			Date:    weekday + " " + label,
			TempMax: tMax,
			TempMin: tMin,
			Code:    code,
			Desc:    weatherDescription(code),
		})
	}

	return wd, nil
}

func germanWeekday(d time.Weekday) string {
	switch d {
	case time.Monday:
		return "Mo"
	case time.Tuesday:
		return "Di"
	case time.Wednesday:
		return "Mi"
	case time.Thursday:
		return "Do"
	case time.Friday:
		return "Fr"
	case time.Saturday:
		return "Sa"
	default:
		return "So"
	}
}

// weatherDescription maps WMO weather codes to German descriptions.
func weatherDescription(code int) string {
	switch {
	case code == 0:
		return "Sonnig"
	case code <= 3:
		return "Bewölkt"
	case code == 45 || code == 48:
		return "Nebel"
	case code >= 51 && code <= 55:
		return "Nieselregen"
	case code >= 61 && code <= 65:
		return "Regen"
	case code >= 71 && code <= 75:
		return "Schnee"
	case code == 77:
		return "Schneekörner"
	case code >= 80 && code <= 82:
		return "Schauer"
	case code >= 85 && code <= 86:
		return "Schneeschauer"
	case code == 95:
		return "Gewitter"
	case code == 96 || code == 99:
		return "Hagel"
	default:
		return "Unbekannt"
	}
}
