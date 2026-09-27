package handler

import (
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"
)

// Contract-id validation for the tracking wizard (issue #140).
//
// The wizard's second step asks the API whether a pasted contract id is
// trackable before the user fills in a label and creates it. v1's
// validateContractID only checks the length and the leading character, which
// accepts any 56-character string; a single mistyped character would sail
// through and only fail much later. This file implements the real StrKey
// check, so typos are caught at the point of entry.

const (
	// strKeyContractVersion is the StrKey version byte for a Soroban contract
	// address. StrKey stores `version << 3`, and contract is version 2, so the
	// raw leading byte is 0x10 — which is why every contract id starts with
	// the base32 character 'C'.
	strKeyContractVersion = byte(0x10)

	// strKeyContractChars is the encoded length: 1 version byte + 32 payload
	// bytes + 2 checksum bytes = 35 bytes, which is 56 base32 characters.
	strKeyContractChars = 56
)

// base32NoPad decodes RFC 4648 base32 without padding, the alphabet StrKey
// uses.
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// crc16XModem computes the checksum StrKey appends. Polynomial 0x1021,
// initial value 0x0000, no reflection, no final XOR.
func crc16XModem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// isValidContractStrKey reports whether id is a well-formed Soroban contract
// StrKey: correct length, decodable base32, the contract version byte, and a
// matching CRC16-XModem checksum. It returns a human-readable reason when the
// id is rejected, so the wizard can tell the user what is wrong.
func isValidContractStrKey(id string) (bool, string) {
	if id == "" {
		return false, "contract id is required"
	}
	if id != strings.ToUpper(id) {
		return false, "contract id must be uppercase"
	}
	if len(id) != strKeyContractChars {
		return false, "contract id must be exactly 56 characters"
	}

	raw, err := base32NoPad.DecodeString(id)
	if err != nil {
		return false, "contract id is not valid base32"
	}
	if len(raw) != 35 {
		return false, "contract id has an unexpected decoded length"
	}
	if raw[0] != strKeyContractVersion {
		return false, "contract id is not a contract address (it must start with 'C')"
	}

	want := uint16(raw[33]) | uint16(raw[34])<<8
	if got := crc16XModem(raw[:33]); got != want {
		return false, "contract id checksum does not match (check for a mistyped character)"
	}
	return true, ""
}

type validateContractResponse struct {
	// Valid is true when the id is a well-formed contract StrKey on a
	// supported network, whether or not it is already tracked.
	Valid bool `json:"valid"`
	// ContractID and Network echo back the submitted values.
	ContractID string `json:"contract_id"`
	Network    string `json:"network"`
	// AlreadyTracked is true when a contract with this id is registered. The
	// wizard treats this as a warning: it points the user at the existing
	// entry instead of creating a duplicate.
	AlreadyTracked bool `json:"already_tracked"`
	// Label is the tracked contract's label when AlreadyTracked is true.
	Label *string `json:"label"`
	// Reason explains a Valid=false result, and is null otherwise.
	Reason *string `json:"reason"`
}

// ValidateContract handles POST /api/v1/contracts/validate.
//
// Body: {"contract_id": "C...", "network": "testnet"}.
//
// It is a read-only pre-flight check for the tracking wizard: it validates the
// StrKey (including checksum) and reports whether the contract is already
// tracked. It never writes. Registration itself stays on
// POST /api/v1/contracts.
func (h *Handler) ValidateContract(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContractID string `json:"contract_id"`
		Network    string `json:"network"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	id := strings.TrimSpace(req.ContractID)
	network := strings.TrimSpace(req.Network)

	resp := validateContractResponse{ContractID: id, Network: network}

	ok, reason := isValidContractStrKey(id)
	if !ok {
		resp.Reason = &reason
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if !validNetworks[network] {
		reason := "network must be one of: testnet, mainnet, futurenet, standalone"
		resp.Reason = &reason
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Valid = true

	// Already-tracked is advisory, not an error: the wizard routes the user to
	// the existing contract rather than failing the step.
	if c, err := h.Store.GetContract(r.Context(), id); err == nil {
		resp.AlreadyTracked = true
		if c.Label != "" {
			label := c.Label
			resp.Label = &label
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
