export interface DecodedScVal {
  type: string;
  value: unknown;
  human: string;
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function readU32(bytes: Uint8Array, offset: number): [number, number] {
  if (offset + 4 > bytes.length) {
    throw new Error("unexpected end of buffer");
  }
  const value =
    (bytes[offset] << 24) |
    (bytes[offset + 1] << 16) |
    (bytes[offset + 2] << 8) |
    bytes[offset + 3];
  return [value >>> 0, offset + 4];
}

function readI32(bytes: Uint8Array, offset: number): [number, number] {
  if (offset + 4 > bytes.length) {
    throw new Error("unexpected end of buffer");
  }
  const value =
    (bytes[offset] << 24) |
    (bytes[offset + 1] << 16) |
    (bytes[offset + 2] << 8) |
    bytes[offset + 3];
  return [value, offset + 4];
}

function readU64(bytes: Uint8Array, offset: number): [bigint, number] {
  if (offset + 8 > bytes.length) {
    throw new Error("unexpected end of buffer");
  }
  const hi =
    (BigInt(bytes[offset]) << 24n) |
    (BigInt(bytes[offset + 1]) << 16n) |
    (BigInt(bytes[offset + 2]) << 8n) |
    BigInt(bytes[offset + 3]);
  const lo =
    (BigInt(bytes[offset + 4]) << 24n) |
    (BigInt(bytes[offset + 5]) << 16n) |
    (BigInt(bytes[offset + 6]) << 8n) |
    BigInt(bytes[offset + 7]);
  return [(hi << 32n) | lo, offset + 8];
}

function readI64(bytes: Uint8Array, offset: number): [bigint, number] {
  const [val, newOffset] = readU64(bytes, offset);
  // Two's complement for negative values
  if (val & (1n << 63n)) {
    return [val - (1n << 64n), newOffset];
  }
  return [val, newOffset];
}

function readU128(bytes: Uint8Array, offset: number): [bigint, number] {
  const [hi, off1] = readU64(bytes, offset);
  const [lo, off2] = readU64(bytes, off1);
  return [(hi << 64n) | lo, off2];
}

function readI128(bytes: Uint8Array, offset: number): [bigint, number] {
  const [val, newOffset] = readU128(bytes, offset);
  if (val & (1n << 127n)) {
    return [val - (1n << 128n), newOffset];
  }
  return [val, newOffset];
}

function readU256(bytes: Uint8Array, offset: number): [bigint, number] {
  const [hi, off1] = readU128(bytes, offset);
  const [lo, off2] = readU128(bytes, off1);
  return [(hi << 128n) | lo, off2];
}

function readI256(bytes: Uint8Array, offset: number): [bigint, number] {
  const [val, newOffset] = readU256(bytes, offset);
  if (val & (1n << 255n)) {
    return [val - (1n << 256n), newOffset];
  }
  return [val, newOffset];
}

// XDR pads variable-length data to a 4-byte boundary
function padded(len: number): number {
  return len + ((4 - (len % 4)) % 4);
}

function readString(bytes: Uint8Array, offset: number): [string, number] {
  const [len, off1] = readU32(bytes, offset);
  if (off1 + len > bytes.length) {
    throw new Error("string length exceeds buffer size");
  }
  const decoder = new TextDecoder();
  const str = decoder.decode(bytes.slice(off1, off1 + len));
  return [str, off1 + padded(len)];
}

function readBytes(bytes: Uint8Array, offset: number): [Uint8Array, number] {
  const [len, off1] = readU32(bytes, offset);
  if (off1 + len > bytes.length) {
    throw new Error("bytes length exceeds buffer size");
  }
  return [bytes.slice(off1, off1 + len), off1 + padded(len)];
}

function readScVal(bytes: Uint8Array, offset: number): [unknown, number] {
  const [discriminant, off] = readI32(bytes, offset);

  switch (discriminant) {
    case 0: {
      // SCV_BOOL - XDR encodes bool as a 4-byte integer
      const [flag, newOff] = readU32(bytes, off);
      return [flag !== 0, newOff];
    }

    case 1: // SCV_VOID
      return [null, off];

    case 2: // SCV_ERROR - union ScError { ScErrorType type; uint32 code } = 8 bytes
      return ["<error>", off + 8];

    case 3: // SCV_U32
      return readU32(bytes, off);

    case 4: // SCV_I32
      return readI32(bytes, off);

    case 5: // SCV_U64
      return readU64(bytes, off);

    case 6: // SCV_I64
      return readI64(bytes, off);

    case 7: // SCV_TIMEPOINT
    case 8: // SCV_DURATION
      return readU64(bytes, off);

    case 9: // SCV_U128
      return readU128(bytes, off);

    case 10: // SCV_I128
      return readI128(bytes, off);

    case 11: // SCV_U256
      return readU256(bytes, off);

    case 12: // SCV_I256
      return readI256(bytes, off);

    case 13: {
      // SCV_BYTES
      const [data, newOff] = readBytes(bytes, off);
      return ["0x" + bytesToHex(data), newOff];
    }

    case 14: // SCV_STRING
    case 15: // SCV_SYMBOL
      return readString(bytes, off);

    case 16: // SCV_VEC
    case 17: {
      // SCV_MAP
      const [len, off1] = readU32(bytes, off);
      if (discriminant === 16) {
        // Vec: each ScVal element requires at least 4 bytes in XDR
        if (len > Math.floor((bytes.length - off1) / 4)) {
          throw new Error("vec length exceeds buffer size");
        }
        const vec: unknown[] = [];
        let currentOff = off1;
        for (let i = 0; i < len; i++) {
          const [val, newOff] = readScVal(bytes, currentOff);
          vec.push(val);
          currentOff = newOff;
        }
        return [vec, currentOff];
      } else {
        // Map: each key-value pair requires at least 8 bytes in XDR
        if (len > Math.floor((bytes.length - off1) / 8)) {
          throw new Error("map length exceeds buffer size");
        }
        const map: Record<string, unknown> = {};
        let currentOff = off1;
        for (let i = 0; i < len; i++) {
          const [key, offKey] = readScVal(bytes, currentOff);
          const [val, offVal] = readScVal(bytes, offKey);
          map[String(key)] = val;
          currentOff = offVal;
        }
        return [map, currentOff];
      }
    }

    case 18: {
      // SCV_ADDRESS
      // XDR union SCAddress: 4-byte ScAddressType discriminant + 32-byte payload.
      const [addrType, typeOff] = readI32(bytes, off);
      const addrBytes = bytes.slice(typeOff, typeOff + 32);
      if (addrType === 0) {
        // ScAddressType::SC_ADDRESS_TYPE_ACCOUNT - G... address
        return ["<account:" + bytesToHex(addrBytes) + ">", typeOff + 32];
      }
      // ScAddressType::SC_ADDRESS_TYPE_CONTRACT - C... address
      return ["<contract:" + bytesToHex(addrBytes) + ">", typeOff + 32];
    }

    case 19: {
      // SCV_CONTRACT_INSTANCE
      // Skip contract instance for now
      return ["<contract_instance>", bytes.length];
    }

    case 20: // SCV_LEDGER_KEY_CONTRACT_INSTANCE
      return ["<ledger_key_instance>", bytes.length];

    case 21: {
      // SCV_LEDGER_KEY_NONCE
      const [val, newOff] = readI64(bytes, off);
      return [val, newOff];
    }

    default:
      return [`<unknown_type:${discriminant}>`, bytes.length];
  }
}

export function decodeScVal(base64Str: string): unknown {
  try {
    const binaryStr = atob(base64Str);
    const bytes = new Uint8Array(binaryStr.length);
    for (let i = 0; i < binaryStr.length; i++) {
      bytes[i] = binaryStr.charCodeAt(i);
    }
    const [result] = readScVal(bytes, 0);
    return result;
  } catch {
    return "<decode_error>";
  }
}

function base64ToBytes(base64Str: string): Uint8Array {
  const binaryStr = atob(base64Str);
  const bytes = new Uint8Array(binaryStr.length);
  for (let i = 0; i < binaryStr.length; i++) {
    bytes[i] = binaryStr.charCodeAt(i);
  }
  return bytes;
}

// ── StrKey encoding (SEP-23) for addresses ──────────────────────────────────

const BASE32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";

function crc16XModem(data: Uint8Array): number {
  let crc = 0;
  for (const byte of data) {
    crc ^= byte << 8;
    for (let i = 0; i < 8; i++) {
      crc = crc & 0x8000 ? ((crc << 1) ^ 0x1021) & 0xffff : (crc << 1) & 0xffff;
    }
  }
  return crc;
}

function base32Encode(data: Uint8Array): string {
  let bits = 0;
  let value = 0;
  let out = "";
  for (const byte of data) {
    value = (value << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      out += BASE32_ALPHABET[(value >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits > 0) {
    out += BASE32_ALPHABET[(value << (5 - bits)) & 31];
  }
  return out;
}

/** Encodes a StrKey: version byte, payload, then a little-endian CRC16. */
export function encodeStrKey(versionByte: number, payload: Uint8Array): string {
  const body = new Uint8Array(1 + payload.length);
  body[0] = versionByte;
  body.set(payload, 1);
  const crc = crc16XModem(body);
  const full = new Uint8Array(body.length + 2);
  full.set(body);
  full[body.length] = crc & 0xff;
  full[body.length + 1] = crc >> 8;
  return base32Encode(full);
}

const STRKEY_ACCOUNT = 6 << 3; // G...
const STRKEY_CONTRACT = 2 << 3; // C...
const STRKEY_MUXED = 12 << 3; // M...
const STRKEY_CLAIMABLE_BALANCE = 1 << 3; // B...
const STRKEY_LIQUIDITY_POOL = 11 << 3; // L...

function readFixed(
  bytes: Uint8Array,
  offset: number,
  len: number
): [Uint8Array, number] {
  if (offset + len > bytes.length) {
    throw new Error("unexpected end of buffer");
  }
  return [bytes.slice(offset, offset + len), offset + len];
}

function readAddress(bytes: Uint8Array, offset: number): [string, number] {
  const [addrType, off] = readI32(bytes, offset);
  switch (addrType) {
    case 0: {
      // SC_ADDRESS_TYPE_ACCOUNT: PublicKey union (only ed25519 = 0)
      const [, keyOff] = readI32(bytes, off);
      const [key, end] = readFixed(bytes, keyOff, 32);
      return [encodeStrKey(STRKEY_ACCOUNT, key), end];
    }
    case 1: {
      // SC_ADDRESS_TYPE_CONTRACT: 32-byte contract id
      const [id, end] = readFixed(bytes, off, 32);
      return [encodeStrKey(STRKEY_CONTRACT, id), end];
    }
    case 2: {
      // SC_ADDRESS_TYPE_MUXED_ACCOUNT: uint64 id, then ed25519 key
      const [idBytes, keyOff] = readFixed(bytes, off, 8);
      const [key, end] = readFixed(bytes, keyOff, 32);
      const payload = new Uint8Array(40);
      payload.set(key);
      payload.set(idBytes, 32);
      return [encodeStrKey(STRKEY_MUXED, payload), end];
    }
    case 3: {
      // SC_ADDRESS_TYPE_CLAIMABLE_BALANCE: ClaimableBalanceID union (v0 = 0)
      const [idType, hashOff] = readI32(bytes, off);
      const [hash, end] = readFixed(bytes, hashOff, 32);
      const payload = new Uint8Array(33);
      payload[0] = idType;
      payload.set(hash, 1);
      return [encodeStrKey(STRKEY_CLAIMABLE_BALANCE, payload), end];
    }
    case 4: {
      // SC_ADDRESS_TYPE_LIQUIDITY_POOL: 32-byte pool id
      const [id, end] = readFixed(bytes, off, 32);
      return [encodeStrKey(STRKEY_LIQUIDITY_POOL, id), end];
    }
    default:
      throw new Error(`unknown address type ${addrType}`);
  }
}

// ── Typed decoding ──────────────────────────────────────────────────────────

const SC_ERROR_TYPES = [
  "Contract",
  "WasmVm",
  "Context",
  "Storage",
  "Object",
  "Crypto",
  "Events",
  "Budget",
  "Value",
  "Auth",
];

const SC_ERROR_CODES = [
  "ArithDomain",
  "IndexBounds",
  "InvalidInput",
  "MissingValue",
  "ExistingValue",
  "ExceededLimit",
  "InvalidAction",
  "InternalError",
  "UnexpectedType",
  "UnexpectedSize",
];

/** A decoded map entry; keys are full ScVals, so they are kept typed. */
export interface DecodedMapEntry {
  key: DecodedScVal;
  value: DecodedScVal;
}

function scalar(type: string, value: unknown, human: string): DecodedScVal {
  return { type, value, human };
}

function bigScalar(type: string, value: bigint): DecodedScVal {
  return { type, value: value.toString(), human: value.toString() };
}

function humanMap(entries: DecodedMapEntry[]): string {
  return `{${entries.map((e) => `${e.key.human}: ${e.value.human}`).join(", ")}}`;
}

function readMapEntries(
  bytes: Uint8Array,
  offset: number
): [DecodedMapEntry[], number] {
  const [len, off] = readU32(bytes, offset);
  // Each key-value pair needs at least 8 bytes.
  if (len > Math.floor((bytes.length - off) / 8)) {
    throw new Error("map length exceeds buffer size");
  }
  const entries: DecodedMapEntry[] = [];
  let current = off;
  for (let i = 0; i < len; i++) {
    const [key, keyEnd] = readTyped(bytes, current);
    const [value, valueEnd] = readTyped(bytes, keyEnd);
    entries.push({ key, value });
    current = valueEnd;
  }
  return [entries, current];
}

function readTyped(bytes: Uint8Array, offset: number): [DecodedScVal, number] {
  const [discriminant, off] = readI32(bytes, offset);

  switch (discriminant) {
    case 0: {
      const [flag, end] = readU32(bytes, off);
      const value = flag !== 0;
      return [scalar("bool", value, value ? "true" : "false"), end];
    }
    case 1:
      return [scalar("void", null, "void"), off];
    case 2: {
      const [errType, codeOff] = readI32(bytes, off);
      const [code, end] = readU32(bytes, codeOff);
      const typeName = SC_ERROR_TYPES[errType] ?? `Unknown${errType}`;
      const codeName =
        errType === 0 ? `#${code}` : (SC_ERROR_CODES[code] ?? `#${code}`);
      return [
        scalar(
          "error",
          { type: typeName, code },
          `Error(${typeName}, ${codeName})`
        ),
        end,
      ];
    }
    case 3: {
      const [value, end] = readU32(bytes, off);
      return [scalar("u32", value, value.toString()), end];
    }
    case 4: {
      const [value, end] = readI32(bytes, off);
      return [scalar("i32", value, value.toString()), end];
    }
    case 5: {
      const [value, end] = readU64(bytes, off);
      return [bigScalar("u64", value), end];
    }
    case 6: {
      const [value, end] = readI64(bytes, off);
      return [bigScalar("i64", value), end];
    }
    case 7: {
      const [value, end] = readU64(bytes, off);
      const seconds = Number(value);
      const iso =
        Number.isSafeInteger(seconds) && seconds * 1000 <= 8.64e15
          ? new Date(seconds * 1000).toISOString()
          : value.toString();
      return [{ type: "timepoint", value: value.toString(), human: iso }, end];
    }
    case 8: {
      const [value, end] = readU64(bytes, off);
      return [
        { type: "duration", value: value.toString(), human: `${value}s` },
        end,
      ];
    }
    case 9: {
      const [value, end] = readU128(bytes, off);
      return [bigScalar("u128", value), end];
    }
    case 10: {
      const [value, end] = readI128(bytes, off);
      return [bigScalar("i128", value), end];
    }
    case 11: {
      const [value, end] = readU256(bytes, off);
      return [bigScalar("u256", value), end];
    }
    case 12: {
      const [value, end] = readI256(bytes, off);
      return [bigScalar("i256", value), end];
    }
    case 13: {
      const [data, end] = readBytes(bytes, off);
      const hex = "0x" + bytesToHex(data);
      return [scalar("bytes", hex, hex), end];
    }
    case 14: {
      const [value, end] = readString(bytes, off);
      return [scalar("string", value, JSON.stringify(value)), end];
    }
    case 15: {
      const [value, end] = readString(bytes, off);
      return [scalar("symbol", value, value), end];
    }
    case 16: {
      const [len, lenEnd] = readU32(bytes, off);
      // Each element needs at least 4 bytes.
      if (len > Math.floor((bytes.length - lenEnd) / 4)) {
        throw new Error("vec length exceeds buffer size");
      }
      const items: DecodedScVal[] = [];
      let current = lenEnd;
      for (let i = 0; i < len; i++) {
        const [item, end] = readTyped(bytes, current);
        items.push(item);
        current = end;
      }
      return [
        {
          type: "vec",
          value: items,
          human: `[${items.map((i) => i.human).join(", ")}]`,
        },
        current,
      ];
    }
    case 17: {
      const [entries, end] = readMapEntries(bytes, off);
      return [{ type: "map", value: entries, human: humanMap(entries) }, end];
    }
    case 18: {
      const [address, end] = readAddress(bytes, off);
      return [scalar("address", address, address), end];
    }
    case 19: {
      // ScContractInstance { ContractExecutable executable; SCMap* storage }
      const [execType, execOff] = readI32(bytes, off);
      let executable: string;
      let current = execOff;
      if (execType === 0) {
        const [hash, end] = readFixed(bytes, execOff, 32);
        executable = `wasm:${bytesToHex(hash)}`;
        current = end;
      } else {
        executable = "stellar_asset";
      }
      const [hasStorage, storageOff] = readU32(bytes, current);
      let storage: DecodedMapEntry[] = [];
      current = storageOff;
      if (hasStorage !== 0) {
        [storage, current] = readMapEntries(bytes, storageOff);
      }
      return [
        {
          type: "contract_instance",
          value: { executable, storage },
          human: `ContractInstance(${executable}, ${humanMap(storage)})`,
        },
        current,
      ];
    }
    case 20:
      return [
        scalar(
          "ledger_key_contract_instance",
          null,
          "LedgerKeyContractInstance"
        ),
        off,
      ];
    case 21: {
      const [value, end] = readI64(bytes, off);
      return [
        {
          type: "ledger_key_nonce",
          value: value.toString(),
          human: `Nonce(${value})`,
        },
        end,
      ];
    }
    default:
      throw new Error(`unknown ScVal type ${discriminant}`);
  }
}

const DECODE_ERROR: DecodedScVal = {
  type: "error",
  value: null,
  human: "<decode_error>",
};

/**
 * Decode a base64 ScVal into a normalized `{ type, value, human }` object.
 *
 * Every ScVal type is supported. Integers wider than 32 bits are returned as
 * decimal strings so they survive JSON; addresses are StrKeys (`G…`, `C…`,
 * `M…`, `B…`, `L…`); vec values are arrays of decoded values and map values
 * are arrays of `{ key, value }` entries. Malformed input decodes to
 * `{ type: "error", value: null, human: "<decode_error>" }`.
 */
export function decode(base64Str: string): DecodedScVal {
  let bytes: Uint8Array;
  try {
    bytes = base64ToBytes(base64Str);
  } catch {
    return DECODE_ERROR;
  }
  if (bytes.length < 4) {
    return DECODE_ERROR;
  }
  try {
    const [value] = readTyped(bytes, 0);
    return value;
  } catch {
    return DECODE_ERROR;
  }
}

// ── Spec-aware decoding ─────────────────────────────────────────────────────

/**
 * A subset of a contract spec type, enough to name values. Mirrors the
 * `ScSpecTypeDef` / `ScSpecUdt*` shapes from a contract's spec.
 */
export type SpecType =
  | { kind: "struct"; name: string; fields: { name: string; type: SpecType }[] }
  | {
      kind: "union";
      name: string;
      cases: { name: string; types?: SpecType[] }[];
    }
  | { kind: "enum"; name: string; cases: { name: string; value: number }[] }
  | { kind: "vec"; element: SpecType }
  | { kind: "map"; key: SpecType; value: SpecType }
  | { kind: "option"; value: SpecType }
  | { kind: "tuple"; elements: SpecType[] }
  | { kind: "primitive" };

/** A decoded value with names applied from a contract spec. */
export interface NamedScVal extends DecodedScVal {
  /** The spec type name (struct, union or enum) the value was matched to. */
  specName?: string;
  /** Matched union case or enum variant name. */
  variant?: string;
}

function applySpec(val: DecodedScVal, spec: SpecType): NamedScVal {
  switch (spec.kind) {
    case "struct": {
      if (val.type === "map") {
        // Named-field structs are maps keyed by field-name symbols.
        const entries = val.value as DecodedMapEntry[];
        const fields: Record<string, NamedScVal> = {};
        for (const field of spec.fields) {
          const entry = entries.find(
            (e) => e.key.type === "symbol" && e.key.value === field.name
          );
          if (!entry) return val;
          fields[field.name] = applySpec(entry.value, field.type);
        }
        const human = `${spec.name} { ${spec.fields
          .map((f) => `${f.name}: ${fields[f.name].human}`)
          .join(", ")} }`;
        return { type: "struct", value: fields, human, specName: spec.name };
      }
      if (val.type === "vec") {
        // Tuple structs are vecs whose fields are named "0", "1", ...
        const items = val.value as DecodedScVal[];
        if (items.length !== spec.fields.length) return val;
        const fields: Record<string, NamedScVal> = {};
        spec.fields.forEach((f, i) => {
          fields[f.name] = applySpec(items[i], f.type);
        });
        const human = `${spec.name}(${spec.fields.map((f) => fields[f.name].human).join(", ")})`;
        return { type: "struct", value: fields, human, specName: spec.name };
      }
      return val;
    }
    case "union": {
      // Unions are vecs: [Symbol(case), ...payload].
      if (val.type !== "vec") return val;
      const items = val.value as DecodedScVal[];
      const head = items[0];
      if (!head || head.type !== "symbol") return val;
      const unionCase = spec.cases.find((c) => c.name === head.value);
      if (!unionCase) return val;
      const types = unionCase.types ?? [];
      const payload = items.slice(1);
      if (payload.length !== types.length) return val;
      const named = payload.map((p, i) => applySpec(p, types[i]));
      const human =
        named.length === 0
          ? `${spec.name}::${unionCase.name}`
          : `${spec.name}::${unionCase.name}(${named.map((n) => n.human).join(", ")})`;
      return {
        type: "union",
        value: named,
        human,
        specName: spec.name,
        variant: unionCase.name,
      };
    }
    case "enum": {
      if (val.type !== "u32") return val;
      const variant = spec.cases.find((c) => c.value === val.value);
      if (!variant) return val;
      return {
        type: "enum",
        value: val.value,
        human: `${spec.name}::${variant.name}`,
        specName: spec.name,
        variant: variant.name,
      };
    }
    case "vec": {
      if (val.type !== "vec") return val;
      const items = (val.value as DecodedScVal[]).map((i) =>
        applySpec(i, spec.element)
      );
      return {
        ...val,
        value: items,
        human: `[${items.map((i) => i.human).join(", ")}]`,
      };
    }
    case "tuple": {
      if (val.type !== "vec") return val;
      const items = val.value as DecodedScVal[];
      if (items.length !== spec.elements.length) return val;
      const named = items.map((item, i) => applySpec(item, spec.elements[i]));
      return {
        ...val,
        value: named,
        human: `(${named.map((n) => n.human).join(", ")})`,
      };
    }
    case "map": {
      if (val.type !== "map") return val;
      const entries = (val.value as DecodedMapEntry[]).map((e) => ({
        key: applySpec(e.key, spec.key),
        value: applySpec(e.value, spec.value),
      }));
      return { ...val, value: entries, human: humanMap(entries) };
    }
    case "option":
      return val.type === "void" ? val : applySpec(val, spec.value);
    case "primitive":
      return val;
  }
}

/**
 * Decode a base64 ScVal and name its parts using a contract spec type.
 *
 * Struct fields, union cases and enum variants get their spec names, and
 * `human` uses Rust-like syntax (`Point { x: 1, y: 2 }`, `Status::Active`).
 * Where the value does not match the spec, that part is returned exactly as
 * {@link decode} would return it, so output never depends on a spec being
 * complete.
 */
export function decodeWithSpec(base64Str: string, spec: SpecType): NamedScVal {
  const val = decode(base64Str);
  return val.type === "error" && val.value === null
    ? val
    : applySpec(val, spec);
}

export function decodeTopic(topicXdr: string[]): unknown[] {
  return topicXdr.map((xdr) => decodeScVal(xdr));
}
