package avflow

import (
	"encoding/json"
	"log/slog"
	"regexp"
)

// secretKeyRE matches JSON field names that typically hold credentials.
// Mirrors platform/workers/router/src/redact.ts SECRET_KEY_RE and pkg/config.
var secretKeyRE = regexp.MustCompile(`(?i)(secret|token|password|credential|apikey|authorization)`)

// LogValue implements slog.LogValuer. Logging a *Job emits a JSON-shaped
// object with credential-bearing fields masked (token, apiKey, …).
func (j *Job) LogValue() slog.Value {
	return slog.AnyValue(jobLogPayload(j))
}

func jobLogPayload(j *Job) any {
	if j == nil {
		return nil
	}
	data, err := json.Marshal(j)
	if err != nil {
		return map[string]string{"marshalError": err.Error()}
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return map[string]string{"unmarshalError": err.Error()}
	}
	redactSecrets(v)
	return v
}

func redactSecrets(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if secretKeyRE.MatchString(k) {
				if s, ok := val.(string); ok {
					x[k] = maskSecret(s)
				} else if val != nil {
					x[k] = "******"
				}
				continue
			}
			redactSecrets(val)
		}
	case []any:
		for i := range x {
			redactSecrets(x[i])
		}
	}
}

// maskSecret shows a recognizable prefix/suffix with a fixed-width mask.
// Matches platform/workers/router/src/redact.ts maskSecret for strings > 8.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:3] + "****" + s[len(s)-3:]
}
