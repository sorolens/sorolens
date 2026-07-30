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
  const value =
    (bytes[offset] << 24) |
    (bytes[offset + 1] << 16) |
    (bytes[offset + 2] << 8) |
    bytes[offset + 3];
  return [value >>> 0, offset + 4];
}

function readI32(bytes: Uint8Array, offset: number): [number, number] {
  const value =
    (bytes[offset] << 24) |
    (bytes[offset + 1] << 16) |
    (bytes[offset + 2] << 8) |
    bytes[offset + 3];
  return [value, offset + 4];
}

function readU64(bytes: Uint8Array, offset: number): [bigint, number] {
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

// XDR pads variable-length data to a 4-byte boundary
function padded(len: number): number {
  return len + ((4 - (len % 4)) % 4);
}

function readString(bytes: Uint8Array, offset: number): [string, number] {
  const [len, off1] = readU32(bytes, offset);
  const decoder = new TextDecoder();
  const str = decoder.decode(bytes.slice(off1, off1 + len));
  return [str, off1 + padded(len)];
}

function readBytes(bytes: Uint8Array, offset: number): [Uint8Array, number] {
  const [len, off1] = readU32(bytes, offset);
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

    case 2: // SCV_ERROR
      return ["<error>", off + 4];

    case 3: // SCV_U32
      return readU32(bytes, off);

    case 4: // SCV_I32
      return readI32(bytes, off);

    case 5: // SCV_U64
      return readU64(bytes, off);

    case 6: // SCV_I64
      return readI64(bytes, off);

    case 7: // SCV_U128
      return readU128(bytes, off);

    case 8: // SCV_I128
      return readI128(bytes, off);

    case 10: // SCV_SYMBOL
    case 12: // SCV_STRING
      return readString(bytes, off);

    case 11: // SCV_BITSET
      return readU128(bytes, off);

    case 13: // SCV_VEC
    case 14: // SCV_MAP
      {
        const [len, off1] = readU32(bytes, off);
        if (discriminant === 13) {
          // Vec
          const vec: unknown[] = [];
          let currentOff = off1;
          for (let i = 0; i < len; i++) {
            const [val, newOff] = readScVal(bytes, currentOff);
            vec.push(val);
            currentOff = newOff;
          }
          return [vec, currentOff];
        } else {
          // Map
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

    case 15: // SCV_BYTES
      {
        const [data, newOff] = readBytes(bytes, off);
        return ["0x" + bytesToHex(data), newOff];
      }

    case 16: // SCV_ADDRESS
      {
        const addrType = bytes[off];
        const addrBytes = bytes.slice(off + 1, off + 33);
        if (addrType === 0) {
          // ScAddressType::SC_ADDRESS_TYPE_ACCOUNT - G... address
          return ["<account:" + bytesToHex(addrBytes) + ">", off + 33];
        }
        // ScAddressType::SC_ADDRESS_TYPE_CONTRACT - C... address
        return ["<contract:" + bytesToHex(addrBytes) + ">", off + 33];
      }

    case 17: // SCV_CONTRACT_INSTANCE
      {
        // Skip contract instance for now
        return ["<contract_instance>", bytes.length];
      }

    case 18: // SCV_LEDGER_KEY_CONTRACT_INSTANCE
      return ["<ledger_key_instance>", bytes.length];

    case 19: // SCV_LEDGER_KEY_NONCE
      {
        const [val, newOff] = readI64(bytes, off);
        return [val, newOff];
      }

    case 20: // SCV_TIME_POINT
    case 21: // SCV_DURATION
      return readU64(bytes, off);

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

/**
 * Decode a base64 ScVal into a normalized `{ type, value, human }` object.
 * Types without a dedicated case yet fall back to the raw decoded value.
 */
export function decode(base64Str: string): DecodedScVal {
  let bytes: Uint8Array;
  try {
    const binaryStr = atob(base64Str);
    bytes = new Uint8Array(binaryStr.length);
    for (let i = 0; i < binaryStr.length; i++) {
      bytes[i] = binaryStr.charCodeAt(i);
    }
  } catch {
    return { type: "error", value: null, human: "<decode_error>" };
  }

  const [discriminant, off] = readI32(bytes, 0);

  switch (discriminant) {
    case 0: {
      // SCV_BOOL - XDR encodes bool as a 4-byte integer
      const [flag] = readU32(bytes, off);
      const value = flag !== 0;
      return { type: "bool", value, human: value ? "true" : "false" };
    }

    case 3: {
      // SCV_U32
      const [value] = readU32(bytes, off);
      return { type: "u32", value, human: value.toString() };
    }

    case 5: {
      // SCV_U64
      const [value] = readU64(bytes, off);
      return { type: "u64", value: value.toString(), human: value.toString() };
    }

    case 10: {
      // SCV_SYMBOL
      const [value] = readString(bytes, off);
      return { type: "symbol", value, human: value };
    }

    default: {
      const value = decodeScVal(base64Str);
      return { type: "unknown", value, human: String(value) };
    }
  }
}

export function decodeTopic(topicXdr: string[]): unknown[] {
  return topicXdr.map((xdr) => decodeScVal(xdr));
}
