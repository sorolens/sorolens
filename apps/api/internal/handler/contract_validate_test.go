package handler_test

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// strKeyEncode builds a valid Soroban contract StrKey from a 32-byte payload,
// mirroring how stellar-core encodes one: version byte, payload, then a
// CRC16-XModem checksum appended little-endian, base32 encoded without padding.
func strKeyEncode(payload [32]byte) string {
	raw := make([]byte, 0, 35)
	raw = append(raw, 0x10) // contract version << 3
	raw = append(raw, payload[:]...)

	var crc uint16
	for _, b := range raw {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	raw = append(raw, byte(crc), byte(crc>>8))

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

func validContractID(fill byte) string {
	var payload [32]byte
	for i := range payload {
		payload[i] = fill
	}
	return strKeyEncode(payload)
}

func postValidate(t *testing.T, srv http.Handler, id, network string) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"contract_id": id, "network": network})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/validate", bytes.NewReader(body))
	// The ContentTypeJSON middleware rejects a write without this header.
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	return w.Code, out
}

// TestValidateContractAcceptsRealStrKey is the happy path: a correctly encoded
// contract address is reported valid.
func TestValidateContractAcceptsRealStrKey(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	id := validContractID(0x01)

	if len(id) != 56 || !strings.HasPrefix(id, "C") {
		t.Fatalf("test fixture %q is not a 56-char C-address", id)
	}

	code, body := postValidate(t, srv, id, "testnet")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if body["valid"] != true {
		t.Fatalf("want valid=true, got %v (reason %v)", body["valid"], body["reason"])
	}
	if body["already_tracked"] != false {
		t.Errorf("want already_tracked=false, got %v", body["already_tracked"])
	}
	if body["reason"] != nil {
		t.Errorf("want null reason for a valid id, got %v", body["reason"])
	}
	if body["label"] != nil {
		t.Errorf("want null label when not tracked, got %v", body["label"])
	}
}

// TestValidateContractRejectsMistypedChecksum is the case the v1 length-only
// check missed: a plausible-looking id with one character changed.
func TestValidateContractRejectsMistypedChecksum(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	id := validContractID(0x02)

	// Flip one payload character to a different valid base32 character.
	idx := 10
	replacement := "A"
	if string(id[idx]) == "A" {
		replacement = "B"
	}
	tampered := id[:idx] + replacement + id[idx+1:]

	code, body := postValidate(t, srv, tampered, "testnet")
	if code != http.StatusOK {
		t.Fatalf("want 200 (validation is a report, not an error), got %d", code)
	}
	if body["valid"] != false {
		t.Fatalf("want valid=false for a tampered id, got %v", body["valid"])
	}
	reason, _ := body["reason"].(string)
	if !strings.Contains(reason, "checksum") {
		t.Errorf("want a checksum explanation, got %q", reason)
	}
}

// TestValidateContractRejectsMalformed ids covers the other rejection paths.
func TestValidateContractRejectsMalformed(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	cases := []struct {
		name     string
		id       string
		contains string
	}{
		{"empty", "", "required"},
		{"too short", "CABC", "56 characters"},
		{"lowercase", strings.ToLower(validContractID(0x03)), "uppercase"},
		{"not base32", "C" + strings.Repeat("1", 55), "base32"},
		{"wrong version", "G" + validContractID(0x04)[1:], "'C'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, body := postValidate(t, srv, tc.id, "testnet")
			if body["valid"] != false {
				t.Fatalf("want valid=false, got %v", body["valid"])
			}
			reason, _ := body["reason"].(string)
			if !strings.Contains(reason, tc.contains) {
				t.Errorf("want reason containing %q, got %q", tc.contains, reason)
			}
		})
	}
}

// TestValidateContractRejectsBadNetwork keeps the network check authoritative.
func TestValidateContractRejectsBadNetwork(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	_, body := postValidate(t, srv, validContractID(0x05), "not-a-network")
	if body["valid"] != false {
		t.Fatalf("want valid=false for an unknown network, got %v", body["valid"])
	}
}

// TestValidateContractReportsAlreadyTracked gives the wizard what it needs to
// redirect instead of creating a duplicate.
func TestValidateContractReportsAlreadyTracked(t *testing.T) {
	ms := store.NewMockStore()
	id := validContractID(0x06)
	if err := ms.UpsertContract(t.Context(), store.Contract{
		ID: id, Network: "testnet", Label: "Existing", Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	_, body := postValidate(t, srv, id, "testnet")
	if body["valid"] != true {
		t.Fatalf("want valid=true, got %v", body["valid"])
	}
	if body["already_tracked"] != true {
		t.Fatalf("want already_tracked=true, got %v", body["already_tracked"])
	}
	if body["label"] != "Existing" {
		t.Errorf("want label Existing, got %v", body["label"])
	}
}

// TestValidateContractIsReadOnly asserts the pre-flight check never writes.
func TestValidateContractIsReadOnly(t *testing.T) {
	ms := store.NewMockStore()
	srv := newTestHandler(ms, true, true)

	id := validContractID(0x07)
	postValidate(t, srv, id, "testnet")

	if _, err := ms.GetContract(t.Context(), id); err == nil {
		t.Error("validate must not register the contract")
	}
}
