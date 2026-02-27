package main

import (
	"embed"
	"html/template"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

//go:embed templates/metrics.html templates/login.html
var metricsTemplateFS embed.FS

type metricsTemplateData struct {
	BundleReceived uint64
	BundleSent     []relayMetric
	BundleFailed   []relayMetric
	RelayLatency   []relayLatencyMetric
	UpdatedAt      string
}

type relayMetric struct {
	Relay string
	Value uint64
}

type relayLatencyMetric struct {
	Relay string
	Count uint64
	Sum   float64
}

func parseMetrics(families []*dto.MetricFamily) *metricsTemplateData {
	data := &metricsTemplateData{
		BundleSent:   []relayMetric{},
		BundleFailed: []relayMetric{},
		RelayLatency: []relayLatencyMetric{},
		UpdatedAt:    time.Now().Format(time.RFC3339),
	}
	for _, f := range families {
		switch f.GetName() {
		case "bundle_received_total":
			for _, m := range f.Metric {
				if m.Counter != nil && m.Counter.Value != nil {
					data.BundleReceived = uint64(*m.Counter.Value)
				}
			}
		case "bundle_sent_total":
			for _, m := range f.Metric {
				if m.Counter != nil && m.Counter.Value != nil {
					relay := getLabel(m, "relay")
					data.BundleSent = append(data.BundleSent, relayMetric{Relay: relay, Value: uint64(*m.Counter.Value)})
				}
			}
		case "bundle_failed_total":
			for _, m := range f.Metric {
				if m.Counter != nil && m.Counter.Value != nil {
					relay := getLabel(m, "relay")
					data.BundleFailed = append(data.BundleFailed, relayMetric{Relay: relay, Value: uint64(*m.Counter.Value)})
				}
			}
		case "relay_latency_ms":
			for _, m := range f.Metric {
				if m.Histogram != nil {
					relay := getLabel(m, "relay")
					var count uint64
					var sum float64
					if m.Histogram.SampleCount != nil {
						count = *m.Histogram.SampleCount
					}
					if m.Histogram.SampleSum != nil {
						sum = *m.Histogram.SampleSum
					}
					data.RelayLatency = append(data.RelayLatency, relayLatencyMetric{Relay: relay, Count: count, Sum: sum})
				}
			}
		}
	}
	return data
}

func getLabel(m *dto.Metric, name string) string {
	for _, l := range m.Label {
		if l.Name != nil && *l.Name == name && l.Value != nil {
			return *l.Value
		}
	}
	return ""
}

var metricsTmpl *template.Template
var loginTmpl *template.Template

func init() {
	metricsTmpl = template.Must(template.ParseFS(metricsTemplateFS, "templates/metrics.html"))
	loginTmpl = template.Must(template.ParseFS(metricsTemplateFS, "templates/login.html"))
}

type loginTemplateData struct {
	Error      string
	LockedUntil string
}

func metricsHandler(auth *authGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth == nil {
			serveMetrics(w, r)
			return
		}
		ip := auth.clientIP(r)
		if cookie, err := r.Cookie(sessionCookieName); err == nil && auth.isValidSession(cookie.Value) {
			serveMetrics(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if auth.isLocked(ip) {
				serveLogin(w, r, "", getLockedUntil(auth, ip))
				return
			}
			password := r.FormValue("password")
			if auth.verifyPassword(password) {
				auth.clearAttempts(ip)
				sessionID, _ := auth.createSession()
				auth.setSession(sessionID)
				http.SetCookie(w, &http.Cookie{
					Name:     sessionCookieName,
					Value:    sessionID,
					Path:     "/",
					HttpOnly: true,
					Secure:   r.TLS != nil,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   int(sessionLifetime.Seconds()),
				})
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return
			}
			auth.recordFailedAttempt(ip)
			if auth.isLocked(ip) {
				serveLogin(w, r, "Too many failed attempts.", getLockedUntil(auth, ip))
				return
			}
			serveLogin(w, r, "Invalid password.", "")
			return
		}
		if auth.isLocked(ip) {
			serveLogin(w, r, "", getLockedUntil(auth, ip))
			return
		}
		serveLogin(w, r, "", "")
	}
}

func getLockedUntil(auth *authGuard, ip string) string {
	auth.mu.RLock()
	defer auth.mu.RUnlock()
	rec := auth.attempts[ip]
	if rec == nil || time.Now().After(rec.lockedUntil) {
		return ""
	}
	return rec.lockedUntil.Format("15:04 MST")
}

func serveLogin(w http.ResponseWriter, r *http.Request, err, lockedUntil string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginTmpl.Execute(w, loginTemplateData{Error: err, LockedUntil: lockedUntil})
}

func serveMetrics(w http.ResponseWriter, r *http.Request) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := parseMetrics(families)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := metricsTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
