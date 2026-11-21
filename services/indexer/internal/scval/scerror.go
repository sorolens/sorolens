package scval

import (
	"encoding/json"
	"fmt"
)

// ScErrorType values (XDR enum SCErrorType, Protocol 20+).
const (
	ScErrorTypeContract uint32 = 0
	ScErrorTypeWasmVm   uint32 = 1
	ScErrorTypeContext  uint32 = 2
	ScErrorTypeStorage  uint32 = 3
	ScErrorTypeObject   uint32 = 4
	ScErrorTypeCrypto   uint32 = 5
	ScErrorTypeEvents   uint32 = 6
	ScErrorTypeBudget   uint32 = 7
	ScErrorTypeValue    uint32 = 8
	ScErrorTypeAuth     uint32 = 9
)

var scErrorTypeNames = map[uint32]string{
	ScErrorTypeContract: "Contract",
	ScErrorTypeWasmVm:   "WasmVm",
	ScErrorTypeContext:  "Context",
	ScErrorTypeStorage:  "Storage",
	ScErrorTypeObject:   "Object",
	ScErrorTypeCrypto:   "Crypto",
	ScErrorTypeEvents:   "Events",
	ScErrorTypeBudget:   "Budget",
	ScErrorTypeValue:    "Value",
	ScErrorTypeAuth:     "Auth",
}

// scErrorCodeNames are the XDR enum SCErrorCode names. They apply to every
// error type except Contract, whose code is defined by the contract itself.
var scErrorCodeNames = map[uint32]string{
	0: "ArithDomain",
	1: "IndexBounds",
	2: "InvalidInput",
	3: "MissingValue",
	4: "ExistingValue",
	5: "ExceededLimit",
	6: "InvalidAction",
	7: "InternalError",
	8: "UnexpectedType",
	9: "UnexpectedSize",
}

// ScError is a decoded XDR SCError, the value of an scvError ScVal. The Soroban
// host reports failures with it, most visibly as the second topic of an
// "error" diagnostic event.
type ScError struct {
	// Type is the SCErrorType discriminant.
	Type uint32
	// Code is the contract-defined error code when Type is Contract, and an
	// SCErrorCode otherwise.
	Code uint32
}

// TypeName returns the SCErrorType name, and false for a type this decoder
// does not know.
func (e ScError) TypeName() (string, bool) {
	n, ok := scErrorTypeNames[e.Type]
	return n, ok
}

// CodeName returns the SCErrorCode name. It returns false for a Contract error,
// whose code has no host-defined name, for an unknown type, whose union arm
// (and so the meaning of the code) is unknown, and for an unknown code.
func (e ScError) CodeName() (string, bool) {
	if _, known := e.TypeName(); !known || e.Type == ScErrorTypeContract {
		return "", false
	}
	n, ok := scErrorCodeNames[e.Code]
	return n, ok
}

// String renders the error the way the Soroban host and stellar-cli print it:
// "Error(Budget, ExceededLimit)", "Error(Contract, #4)". A type or code this
// decoder does not know is printed as its raw number, "Error(#12, #3)", so the
// value is never lost.
func (e ScError) String() string {
	typ, ok := e.TypeName()
	if !ok {
		typ = fmt.Sprintf("#%d", e.Type)
	}
	code, ok := e.CodeName()
	if !ok {
		code = fmt.Sprintf("#%d", e.Code)
	}
	return fmt.Sprintf("Error(%s, %s)", typ, code)
}

// MarshalJSON stores an error the same way it is printed, so a decoded event
// topic or value reads "Error(Auth, InvalidAction)" rather than a bare struct.
func (e ScError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}
