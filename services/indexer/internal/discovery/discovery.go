// Package discovery finds Soroban contract deployments in transaction
// envelopes so contracts deployed by a watched account can be tracked
// automatically (issue #123).
//
// Like internal/wasm it decodes only the XDR it needs instead of importing
// the stellar/go SDK. A Soroban transaction carries exactly one operation, so
// the decoder reads the envelope up to that operation and stops; any
// transaction that is not a single InvokeHostFunction op is skipped.
//
// Wire layout handled here (Protocol 20+):
//
//	TransactionEnvelope: union EnvelopeType
//	  ENVELOPE_TYPE_TX (2)         -> TransactionV1Envelope { Transaction tx; ... }
//	  ENVELOPE_TYPE_TX_FEE_BUMP (5) -> FeeBumpTransaction { MuxedAccount feeSource;
//	                                   int64 fee; union { ENVELOPE_TYPE_TX: TransactionV1Envelope } }
//	Transaction:
//	  MuxedAccount sourceAccount; uint32 fee; int64 seqNum; Preconditions cond;
//	  Memo memo; Operation operations<100>; ...
//	Operation: MuxedAccount* sourceAccount; OperationBody body
//	  body INVOKE_HOST_FUNCTION (24): HostFunction
//	    CREATE_CONTRACT (1) / CREATE_CONTRACT_V2 (3): ContractIDPreimage first
//
// The deployed contract ID is sha256(HashIDPreimage{ENVELOPE_TYPE_CONTRACT_ID,
// sha256(networkPassphrase), contractIDPreimage}); the preimage bytes are
// hashed exactly as they appear in the envelope.
package discovery

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

// Network passphrases used to derive contract IDs.
const (
	PassphraseTestnet    = "Test SDF Network ; September 2015"
	PassphraseMainnet    = "Public Global Stellar Network ; September 2015"
	PassphraseFuturenet  = "Test SDF Future Network ; October 2022"
	PassphraseStandalone = "Standalone Network ; February 2017"
)

// Passphrase returns the network passphrase for a Sorolens network name. An
// empty name is the single-network default, testnet.
func Passphrase(network string) (string, bool) {
	switch network {
	case "", "testnet":
		return PassphraseTestnet, true
	case "mainnet":
		return PassphraseMainnet, true
	case "futurenet":
		return PassphraseFuturenet, true
	case "standalone":
		return PassphraseStandalone, true
	}
	return "", false
}

// XDR discriminants used by this package.
const (
	envelopeTypeTx         = 2
	envelopeTypeTxFeeBump  = 5
	envelopeTypeContractID = 8
	keyTypeEd25519         = 0
	keyTypeMuxedEd25519    = 0x100
	precondNone            = 0
	precondTime            = 1
	precondV2              = 2
	memoNone               = 0
	memoText               = 1
	memoID                 = 2
	memoHash               = 3
	memoReturn             = 4
	opInvokeHostFunction   = 24
	hostFnCreateContract   = 1
	hostFnCreateContractV2 = 3
	preimageFromAddress    = 0
	preimageFromAsset      = 1
	scAddressAccount       = 0
	scAddressContract      = 1
	scAddressMuxedAccount  = 2
	assetNative            = 0
	assetCreditAlphanum4   = 1
	assetCreditAlphanum12  = 2
	signerKeyEd25519       = 0
	signerKeyPreAuthTx     = 1
	signerKeyHashX         = 2
	signerKeySignedPayload = 3
	strkeyVersionAccount   = 6 << 3
	strkeyVersionContract  = 2 << 3
	maxSignedPayloadLen    = 64
	maxExtraSigners        = 2
)

// Deployment is one contract creation found in a transaction.
type Deployment struct {
	// ContractID is the deployed contract's strkey (C...).
	ContractID string
	// Source is the account that submitted the create operation: the
	// operation source if set, else the transaction source (G...).
	Source string
	// Deployer is the address in a from-address preimage (G... or C...), or
	// empty for a Stellar Asset Contract created from an asset.
	Deployer string
}

var errMalformed = errors.New("discovery: malformed XDR")

// Deployments returns the contracts created by a base64 transaction envelope
// on the network identified by passphrase. Envelopes that do not deploy a
// contract return (nil, nil); only undecodable envelopes return an error.
func Deployments(envelopeXDR, passphrase string) ([]Deployment, error) {
	raw, err := base64.StdEncoding.DecodeString(envelopeXDR)
	if err != nil {
		return nil, fmt.Errorf("discovery: decode envelope: %w", err)
	}
	d := &decoder{b: raw}

	switch envType := d.u32(); envType {
	case envelopeTypeTx:
	case envelopeTypeTxFeeBump:
		d.muxedAccount() // fee source
		d.skip(8)        // fee
		if inner := d.u32(); inner != envelopeTypeTx {
			return nil, d.errOr(nil)
		}
	default:
		// TX_V0 envelopes predate Soroban and cannot deploy contracts.
		return nil, d.errOr(nil)
	}

	txSource := d.muxedAccount()
	d.skip(4 + 8) // fee, seqNum
	d.preconditions()
	d.memo()
	if n := d.u32(); d.err != nil || n != 1 {
		// Soroban transactions carry exactly one operation.
		return nil, d.errOr(nil)
	}

	source := txSource
	if d.present() {
		source = d.muxedAccount()
	}
	if op := d.u32(); d.err != nil || op != opInvokeHostFunction {
		return nil, d.errOr(nil)
	}
	if fn := d.u32(); d.err != nil || (fn != hostFnCreateContract && fn != hostFnCreateContractV2) {
		return nil, d.errOr(nil)
	}

	start := d.off
	deployer := d.contractIDPreimage()
	if d.err != nil {
		return nil, d.err
	}
	preimage := d.b[start:d.off]

	return []Deployment{{
		ContractID: contractID(passphrase, preimage),
		Source:     encodeStrkey(strkeyVersionAccount, source),
		Deployer:   deployer,
	}}, nil
}

// contractID hashes HashIDPreimage{ENVELOPE_TYPE_CONTRACT_ID, networkID,
// preimage} into the contract's strkey.
func contractID(passphrase string, preimage []byte) string {
	networkID := sha256.Sum256([]byte(passphrase))
	buf := make([]byte, 0, 4+32+len(preimage))
	buf = binary.BigEndian.AppendUint32(buf, envelopeTypeContractID)
	buf = append(buf, networkID[:]...)
	buf = append(buf, preimage...)
	id := sha256.Sum256(buf)
	return encodeStrkey(strkeyVersionContract, id[:])
}

// encodeStrkey renders a 32-byte key as a Stellar strkey:
// base32(version || key || crc16-xmodem little-endian), unpadded.
func encodeStrkey(version byte, key []byte) string {
	payload := make([]byte, 0, 35)
	payload = append(payload, version)
	payload = append(payload, key...)
	crc := crc16XModem(payload)
	payload = append(payload, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(payload)
}

func crc16XModem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ---- XDR cursor -------------------------------------------------------------

// decoder is a bounds-checked XDR reader. After the first error every read
// returns zero values, so callers check d.err once at decision points.
type decoder struct {
	b   []byte
	off int
	err error
}

func (d *decoder) errOr(err error) error {
	if d.err != nil {
		return d.err
	}
	return err
}

func (d *decoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n < 0 || len(d.b)-d.off < n {
		d.err = errMalformed
		return nil
	}
	out := d.b[d.off : d.off+n]
	d.off += n
	return out
}

func (d *decoder) skip(n int) { d.take(n) }

func (d *decoder) u32() uint32 {
	b := d.take(4)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}

// present reads an XDR optional flag.
func (d *decoder) present() bool { return d.u32() != 0 }

// varOpaque skips a variable-length opaque/string with the given max length,
// including its padding to a 4-byte boundary.
func (d *decoder) varOpaque(max uint32) {
	n := d.u32()
	if d.err == nil && n > max {
		d.err = errMalformed
		return
	}
	d.skip(int(n+3) &^ 3)
}

// muxedAccount reads a MuxedAccount and returns its ed25519 key.
func (d *decoder) muxedAccount() []byte {
	switch d.u32() {
	case keyTypeEd25519:
		return d.take(32)
	case keyTypeMuxedEd25519:
		d.skip(8) // id
		return d.take(32)
	}
	if d.err == nil {
		d.err = errMalformed
	}
	return nil
}

// accountID reads a PublicKey (only ed25519 exists).
func (d *decoder) accountID() []byte {
	if t := d.u32(); d.err == nil && t != keyTypeEd25519 {
		d.err = errMalformed
		return nil
	}
	return d.take(32)
}

func (d *decoder) preconditions() {
	switch d.u32() {
	case precondNone:
	case precondTime:
		d.skip(16)
	case precondV2:
		if d.present() {
			d.skip(16) // timeBounds
		}
		if d.present() {
			d.skip(8) // ledgerBounds
		}
		if d.present() {
			d.skip(8) // minSeqNum
		}
		d.skip(8 + 4) // minSeqAge, minSeqLedgerGap
		n := d.u32()
		if d.err == nil && n > maxExtraSigners {
			d.err = errMalformed
			return
		}
		for i := uint32(0); i < n && d.err == nil; i++ {
			d.signerKey()
		}
	default:
		if d.err == nil {
			d.err = errMalformed
		}
	}
}

func (d *decoder) signerKey() {
	switch d.u32() {
	case signerKeyEd25519, signerKeyPreAuthTx, signerKeyHashX:
		d.skip(32)
	case signerKeySignedPayload:
		d.skip(32)
		d.varOpaque(maxSignedPayloadLen)
	default:
		if d.err == nil {
			d.err = errMalformed
		}
	}
}

func (d *decoder) memo() {
	switch d.u32() {
	case memoNone:
	case memoText:
		d.varOpaque(28)
	case memoID:
		d.skip(8)
	case memoHash, memoReturn:
		d.skip(32)
	default:
		if d.err == nil {
			d.err = errMalformed
		}
	}
}

// contractIDPreimage reads a ContractIDPreimage and returns the deployer
// address strkey for from-address preimages ("" for from-asset).
func (d *decoder) contractIDPreimage() string {
	switch d.u32() {
	case preimageFromAddress:
		addr := d.scAddress()
		d.skip(32) // salt
		return addr
	case preimageFromAsset:
		d.asset()
		return ""
	}
	if d.err == nil {
		d.err = errMalformed
	}
	return ""
}

func (d *decoder) scAddress() string {
	switch d.u32() {
	case scAddressAccount:
		if key := d.accountID(); key != nil {
			return encodeStrkey(strkeyVersionAccount, key)
		}
	case scAddressContract:
		if key := d.take(32); key != nil {
			return encodeStrkey(strkeyVersionContract, key)
		}
	case scAddressMuxedAccount:
		d.skip(8) // id
		if key := d.take(32); key != nil {
			return encodeStrkey(strkeyVersionAccount, key)
		}
	default:
		if d.err == nil {
			d.err = errMalformed
		}
	}
	return ""
}

func (d *decoder) asset() {
	switch d.u32() {
	case assetNative:
	case assetCreditAlphanum4:
		d.skip(4)
		d.accountID()
	case assetCreditAlphanum12:
		d.skip(12)
		d.accountID()
	default:
		if d.err == nil {
			d.err = errMalformed
		}
	}
}
