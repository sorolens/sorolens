package handler

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func sampleReport() store.MonthlySLA {
	first := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 2, 28, 23, 0, 0, 0, time.UTC)
	return store.MonthlySLA{
		ContractID:           "CABCDEFGHIJKLMNOPQRSTUVWXYZ234567ABCDEFGHIJKLMNOPQRSTUVWX",
		Month:                "2026-02",
		UptimePct:            99.95,
		TotalChecks:          2000,
		HealthyChecks:        1999,
		Incidents:            1,
		MTTRSeconds:          300,
		TotalDowntimeSeconds: 300,
		LongestOutageSeconds: 300,
		CriticalAlerts:       0,
		WarningAlerts:        2,
		InfoAlerts:           1,
		TotalAlerts:          3,
		FirstCheck:           &first,
		LastCheck:            &last,
	}
}

func TestSignReportIsStableAndKeyDependent(t *testing.T) {
	payload := []byte("uptime_pct=99.9500\n")

	a := signReport([]byte("key-one"), payload)
	b := signReport([]byte("key-one"), payload)
	if a != b {
		t.Fatalf("signature is not deterministic: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("HMAC-SHA256 hex signature should be 64 chars, got %d", len(a))
	}
	if c := signReport([]byte("key-two"), payload); c == a {
		t.Fatal("different keys produced the same signature")
	}
	if d := signReport([]byte("key-one"), []byte("uptime_pct=1.0000\n")); d == a {
		t.Fatal("different payloads produced the same signature")
	}
	if e := signReport(nil, payload); e != "" {
		t.Fatalf("no key should mean no signature, got %q", e)
	}
}

func TestRenderReportTextIsCanonical(t *testing.T) {
	rep := sampleReport()
	got := renderReportText(rep)

	// The signature covers this exact rendering, so it must be stable.
	for _, want := range []string{
		"contract_id=" + rep.ContractID,
		"month=2026-02",
		"uptime_pct=99.9500",
		"mttr_seconds=300.00",
		"total_checks=2000",
		"ongoing_outage=false",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("canonical text missing %q\n%s", want, got)
		}
	}
}

func TestRenderReportCSVRoundTrips(t *testing.T) {
	body, err := renderReportCSV(sampleReport())
	if err != nil {
		t.Fatalf("renderReportCSV: %v", err)
	}

	records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected a header and rows, got %d records", len(records))
	}
	if records[0][0] != "metric" || records[0][1] != "value" {
		t.Fatalf("unexpected header: %v", records[0])
	}

	values := make(map[string]string, len(records))
	for _, r := range records[1:] {
		if len(r) != 2 {
			t.Fatalf("row is not metric,value: %v", r)
		}
		values[r[0]] = r[1]
	}
	for k, want := range map[string]string{
		"month":          "2026-02",
		"uptime_pct":     "99.9500",
		"incidents":      "1",
		"mttr_seconds":   "300.00",
		"ongoing_outage": "false",
		"critical_alerts": "0",
	} {
		if values[k] != want {
			t.Fatalf("csv[%s] = %q, want %q", k, values[k], want)
		}
	}
}

func TestRenderReportCSVEscapesContractIDThatLooksNumeric(t *testing.T) {
	rep := sampleReport()
	rep.ContractID = "0123456789"
	body, err := renderReportCSV(rep)
	if err != nil {
		t.Fatalf("renderReportCSV: %v", err)
	}
	records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("not valid CSV: %v", err)
	}
	for _, r := range records[1:] {
		if r[0] == "contract_id" && r[1] != "0123456789" {
			t.Fatalf("contract_id round-trip = %q", r[1])
		}
	}
}

func TestRenderReportPDFHasValidStructure(t *testing.T) {
	body, err := renderReportPDF(sampleReport(), "deadbeef")
	if err != nil {
		t.Fatalf("renderReportPDF: %v", err)
	}

	if !bytes.HasPrefix(body, []byte("%PDF-1.4")) {
		t.Fatal("missing PDF header")
	}
	if !bytes.HasSuffix(body, []byte("%%EOF\n")) {
		t.Fatal("missing EOF trailer")
	}
	if !bytes.Contains(body, []byte("xref")) || !bytes.Contains(body, []byte("startxref")) {
		t.Fatal("missing cross-reference table")
	}
	if !bytes.Contains(body, []byte("/Type /Catalog")) {
		t.Fatal("missing catalog")
	}
	if !bytes.Contains(body, []byte("deadbeef")) {
		t.Fatal("signature was not embedded in the document")
	}
	if !bytes.Contains(body, []byte("/Keywords")) {
		t.Fatal("signature was not placed in the Info dictionary")
	}

	// The whole file hinges on the xref offsets being byte-exact, so verify
	// each one actually points at its object header.
	lines := strings.Split(string(body), "\n")
	xrefStart := -1
	for i, l := range lines {
		if l == "xref" {
			xrefStart = i
			break
		}
	}
	if xrefStart < 0 {
		t.Fatal("no xref keyword")
	}
	header := strings.Fields(lines[xrefStart+1]) // "0 N"
	if len(header) != 2 {
		t.Fatalf("malformed xref header: %q", lines[xrefStart+1])
	}
	count, err := strconv.Atoi(header[1])
	if err != nil {
		t.Fatalf("malformed xref count: %v", err)
	}

	// First entry (object 0) is the free-list head.
	for objNum := 1; objNum < count; objNum++ {
		entry := lines[xrefStart+2+objNum]
		if len(entry) < 18 {
			t.Fatalf("xref entry %d too short: %q", objNum, entry)
		}
		off, err := strconv.Atoi(strings.TrimSpace(entry[:10]))
		if err != nil {
			t.Fatalf("xref entry %d offset unparseable: %q", objNum, entry)
		}
		want := fmt.Sprintf("%d 0 obj", objNum)
		if off < 0 || off >= len(body) {
			t.Fatalf("object %d offset %d out of range", objNum, off)
		}
		if !strings.HasPrefix(string(body[off:]), want) {
			t.Fatalf("xref offset for object %d does not point at %q", objNum, want)
		}
	}
}

func TestRenderReportPDFPaginatesLongContent(t *testing.T) {
	var doc pdfDoc
	lines := make([]string, pdfLinesPerPage+5)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	doc.AddLines(lines)

	if len(doc.pages) != 2 {
		t.Fatalf("got %d pages, want 2 for %d lines", len(doc.pages), len(lines))
	}
	if len(doc.pages[0]) != pdfLinesPerPage {
		t.Fatalf("first page has %d lines, want %d", len(doc.pages[0]), pdfLinesPerPage)
	}
	if !bytes.Contains(doc.Bytes(), []byte("Page 2 of 2")) {
		t.Fatal("expected a page-number footer")
	}
}

func TestEscapePDFTextNeutralisesDelimiters(t *testing.T) {
	got := escapePDFText(`a(b)c\d`)
	want := `a\(b\)c\\d`
	if got != want {
		t.Fatalf("escapePDFText = %q, want %q", got, want)
	}
}

func TestRenderSLABadgeColourThresholds(t *testing.T) {
	cases := []struct {
		name    string
		uptime  float64
		data    bool
		wantHex string
		wantTxt string
	}{
		{"met", 99.95, true, "#2ea44f", "99.950%"},
		{"at risk", 99.30, true, "#d29922", "99.300%"},
		{"missed", 95.00, true, "#d73a49", "95.000%"},
		{"no data", 0, false, "#9ca3af", "no data"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := store.MonthlySLA{Month: "2026-02", UptimePct: tc.uptime}
			if tc.data {
				rep.TotalChecks = 10
			}
			svg := renderSLABadge(rep)

			if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
				t.Fatalf("not an SVG document: %s", svg)
			}
			if !strings.Contains(svg, tc.wantHex) {
				t.Fatalf("expected colour %s for %v%%:\n%s", tc.wantHex, tc.uptime, svg)
			}
			if !strings.Contains(svg, tc.wantTxt) {
				t.Fatalf("expected text %q:\n%s", tc.wantTxt, svg)
			}
			if !strings.Contains(svg, "SLA 2026-02") {
				t.Fatal("missing label")
			}
		})
	}
}

func TestRenderSLABadgeIsWellFormedXML(t *testing.T) {
	svg := renderSLABadge(sampleReport())
	// A cheap well-formedness check: tags are balanced for the elements used.
	// Count "<tag>" and "<tag " as opens (the latter covers attributed tags like
	// <svg xmlns=...>); self-closing elements such as <rect .../> use neither.
	for _, tag := range []string{"svg", "g", "title"} {
		open := strings.Count(svg, "<"+tag+">") + strings.Count(svg, "<"+tag+" ")
		close := strings.Count(svg, "</"+tag+">")
		if open != close {
			t.Fatalf("unbalanced <%s>: %d open, %d close\n%s",
				tag, open, close, svg)
		}
	}
}

func TestReportFilenameIsSanitised(t *testing.T) {
	got := reportFilename("CABCDEFGHIJKLMNOPQRSTUVWXYZ234567", "2026-02", "pdf")
	if got != "sorolens-sla-CABCDEFGHIJ-2026-02.pdf" {
		t.Fatalf("filename = %q", got)
	}
}

func TestBadgeCacheControlCapsCurrentMonth(t *testing.T) {
	current := store.CurrentMonth(time.Now())
	if v := badgeCacheControl(current); !strings.Contains(v, "max-age=300") {
		t.Fatalf("current month should have a short TTL, got %q", v)
	}
	if v := badgeCacheControl("2020-01"); !strings.Contains(v, "max-age=86400") {
		t.Fatalf("a closed month should cache longer, got %q", v)
	}
}

func TestFormatSeconds(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0, "0s"},
		{-5, "0s"},
		{45, "45s"},
		{300, "5.0m"},
		{7200, "2.0h"},
	} {
		if got := formatSeconds(tc.in); got != tc.want {
			t.Fatalf("formatSeconds(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSLAVerdict(t *testing.T) {
	var noData store.MonthlySLA
	if got := slaVerdict(noData); !strings.Contains(got, "no data") {
		t.Fatalf("verdict = %q", got)
	}
	ongoing := store.MonthlySLA{TotalChecks: 10, UptimePct: 50, OngoingOutage: true}
	if got := slaVerdict(ongoing); !strings.Contains(got, "DEGRADED") {
		t.Fatalf("verdict = %q", got)
	}
}

// Guard against the canonical text ever growing a field that is not covered by
// the signature, which would silently make the signature cover less than the
// report contains.
func TestCanonicalTextCoversEveryMetric(t *testing.T) {
	text := renderReportText(sampleReport())
	for _, key := range []string{
		"contract_id", "month", "uptime_pct", "total_checks", "healthy_checks",
		"incidents", "mttr_seconds", "total_downtime_seconds",
		"longest_outage_seconds", "ongoing_outage", "critical_alerts",
		"warning_alerts", "info_alerts", "total_alerts", "first_check", "last_check",
	} {
		matched, err := regexp.MatchString(`(?m)^`+key+`=`, text)
		if err != nil {
			t.Fatalf("bad pattern: %v", err)
		}
		if !matched {
			t.Fatalf("canonical text is missing %q, so it is not signed:\n%s", key, text)
		}
	}
}
