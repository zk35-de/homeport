package i18n

import (
	"embed"
	"encoding/json"
	"log/slog"
	"strings"
)

// translations[lang][key] = value
var translations = map[string]map[string]string{}

// Load lädt alle JSON-Dateien aus assets/i18n/ in den Speicher.
// Muss einmal beim Start aufgerufen werden (in InitTemplates oder main).
func Load(fs embed.FS) {
	entries, err := fs.ReadDir("i18n")
	if err != nil {
		slog.Error("i18n: cannot read i18n dir", "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		data, err := fs.ReadFile("i18n/" + e.Name())
		if err != nil {
			slog.Error("i18n: cannot read file", "file", e.Name(), "err", err)
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			slog.Error("i18n: cannot parse file", "file", e.Name(), "err", err)
			continue
		}
		translations[lang] = m
		slog.Info("i18n: loaded strings", "count", len(m), "lang", lang)
	}
}

// Translator is an embeddable type that exposes a .T method for templates.
// Go 1.25 html/template no longer allows calling function-valued struct fields;
// only real methods work. Embed this in template data structs so {{.T "key"}} works.
type Translator struct {
	tFunc func(string) string
}

// NewTranslator creates a Translator for the given language.
func NewTranslator(lang string) Translator {
	return Translator{tFunc: tFunc(lang)}
}

// T translates a key. Safe to call on zero-value Translator (returns key as-is).
func (tr Translator) T(key string) string {
	if tr.tFunc == nil {
		return key
	}
	return tr.tFunc(key)
}

// tFunc returns the internal translation function for a language.
func tFunc(lang string) func(string) string {
	m, ok := translations[lang]
	if !ok {
		m = translations["de"]
	}
	fallback := translations["de"]
	return func(key string) string {
		if v, ok := m[key]; ok {
			return v
		}
		if v, ok := fallback[key]; ok {
			return v
		}
		return key
	}
}

// T gibt einen Translator-func zurück (für nicht-Template-Nutzung wie 404-Handler).
// Unbekannte Keys geben den Key selbst zurück (nie leer, nie Crash).
// Unbekannte Sprachen fallen auf "de" zurück.
func T(lang string) func(string) string {
	return tFunc(lang)
}

// SupportedLanguages gibt alle geladenen Sprachcodes zurück.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(translations))
	for k := range translations {
		langs = append(langs, k)
	}
	return langs
}
