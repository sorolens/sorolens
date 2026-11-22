import { describe, it, expect } from "vitest";
import { decode, decodeScVal, decodeTopic } from "./index";

// Helpers to encode primitive Soroban ScVal XDR for round-trip property tests.
function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function writeU32(arr: number[], val: number): void {
  arr.push((val >>> 24) & 0xff, (val >>> 16) & 0xff, (val >>> 8) & 0xff, val & 0xff);
}

function writeI32(arr: number[], val: number): void {
  arr.push((val >> 24) & 0xff, (val >> 16) & 0xff, (val >> 8) & 0xff, val & 0xff);
}

function writeU64(arr: number[], val: bigint): void {
  const hi = Number((val >> 32n) & 0xffffffffn);
  const lo = Number(val & 0xffffffffn);
  writeU32(arr, hi);
  writeU32(arr, lo);
}

function writeI64(arr: number[], val: bigint): void {
  const uval = val < 0n ? val + (1n << 64n) : val;
  writeU64(arr, uval);
}

function encodeBool(val: boolean): string {
  const arr: number[] = [];
  writeI32(arr, 0); // SCV_BOOL
  writeU32(arr, val ? 1 : 0);
  return bytesToBase64(new Uint8Array(arr));
}

function encodeU32(val: number): string {
  const arr: number[] = [];
  writeI32(arr, 3); // SCV_U32
  writeU32(arr, val);
  return bytesToBase64(new Uint8Array(arr));
}

function encodeI32(val: number): string {
  const arr: number[] = [];
  writeI32(arr, 4); // SCV_I32
  writeI32(arr, val);
  return bytesToBase64(new Uint8Array(arr));
}

function encodeU64(val: bigint): string {
  const arr: number[] = [];
  writeI32(arr, 5); // SCV_U64
  writeU64(arr, val);
  return bytesToBase64(new Uint8Array(arr));
}

function encodeI64(val: bigint): string {
  const arr: number[] = [];
  writeI32(arr, 6); // SCV_I64
  writeI64(arr, val);
  return bytesToBase64(new Uint8Array(arr));
}

function encodeSymbol(str: string): string {
  const arr: number[] = [];
  writeI32(arr, 15); // SCV_SYMBOL
  const strBytes = new TextEncoder().encode(str);
  writeU32(arr, strBytes.length);
  arr.push(...strBytes);
  const pad = (4 - (strBytes.length % 4)) % 4;
  for (let i = 0; i < pad; i++) {
    arr.push(0);
  }
  return bytesToBase64(new Uint8Array(arr));
}

function encodeBytes(bytes: Uint8Array): string {
  const arr: number[] = [];
  writeI32(arr, 13); // SCV_BYTES
  writeU32(arr, bytes.length);
  arr.push(...bytes);
  const pad = (4 - (bytes.length % 4)) % 4;
  for (let i = 0; i < pad; i++) {
    arr.push(0);
  }
  return bytesToBase64(new Uint8Array(arr));
}

function encodeVecU32(vals: number[]): string {
  const arr: number[] = [];
  writeI32(arr, 16); // SCV_VEC
  writeU32(arr, vals.length);
  for (const v of vals) {
    writeI32(arr, 3); // SCV_U32
    writeU32(arr, v);
  }
  return bytesToBase64(new Uint8Array(arr));
}

// Deterministic pseudo-random number generator for reproducible property tests
class SeededRandom {
  private seed: number;
  constructor(seed = 123456789) {
    this.seed = seed;
  }
  next(): number {
    this.seed = (this.seed * 1664525 + 1013904223) >>> 0;
    return this.seed / 0x100000000;
  }
  nextInt(min: number, max: number): number {
    return Math.floor(this.next() * (max - min + 1)) + min;
  }
  nextBigInt(bits: number): bigint {
    let result = 0n;
    for (let i = 0; i < bits; i += 32) {
      const chunk = BigInt(Math.floor(this.next() * 0x100000000));
      result = (result << 32n) | chunk;
    }
    const mask = (1n << BigInt(bits)) - 1n;
    return result & mask;
  }
  nextString(maxLen = 30): string {
    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_:-";
    const len = this.nextInt(0, maxLen);
    let s = "";
    for (let i = 0; i < len; i++) {
      s += chars[this.nextInt(0, chars.length - 1)];
    }
    return s;
  }
  nextBytes(maxLen = 64): Uint8Array {
    const len = this.nextInt(0, maxLen);
    const b = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      b[i] = this.nextInt(0, 255);
    }
    return b;
  }
}

describe("XDR Property Tests", () => {
  const rng = new SeededRandom(42);

  describe("Crash-freedom invariant", () => {
    it("never throws on arbitrary random string inputs", () => {
      for (let i = 0; i < 500; i++) {
        let input: string;
        const kind = rng.nextInt(0, 5);
        if (kind === 0) {
          input = rng.nextString(rng.nextInt(0, 100));
        } else if (kind === 1) {
          input = bytesToBase64(rng.nextBytes(rng.nextInt(0, 128)));
        } else if (kind === 2) {
          // Corrupted base64
          const valid = bytesToBase64(rng.nextBytes(rng.nextInt(1, 32)));
          input = valid.slice(0, rng.nextInt(0, valid.length - 1)) + "!";
        } else if (kind === 3) {
          // Truncated valid base64
          const valid = encodeU32(rng.nextInt(0, 1000));
          input = valid.slice(0, rng.nextInt(0, valid.length - 1));
        } else {
          // Large discriminant or large length header
          const raw: number[] = [];
          writeI32(raw, rng.nextInt(-100, 100));
          writeU32(raw, rng.nextInt(0, 0xffffffff));
          input = bytesToBase64(new Uint8Array(raw));
        }

        expect(() => {
          const res = decode(input);
          expect(res).toBeDefined();
          expect(typeof res.type).toBe("string");
          expect(typeof res.human).toBe("string");
        }).not.toThrow();

        expect(() => {
          decodeScVal(input);
        }).not.toThrow();

        expect(() => {
          const topics = decodeTopic([input, input]);
          expect(topics).toHaveLength(2);
        }).not.toThrow();
      }
    });
  });

  describe("Round-trip properties", () => {
    it("correctly round-trips all boolean values", () => {
      for (const b of [true, false]) {
        const encoded = encodeBool(b);
        expect(decodeScVal(encoded)).toBe(b);
        expect(decode(encoded)).toEqual({
          type: "bool",
          value: b,
          human: b ? "true" : "false",
        });
      }
    });

    it("correctly round-trips random unsigned 32-bit integers", () => {
      const boundaryU32s = [0, 1, 2, 42, 255, 65535, 16777215, 2147483647, 4294967295];
      for (const val of boundaryU32s) {
        const encoded = encodeU32(val);
        expect(decodeScVal(encoded)).toBe(val);
        expect(decode(encoded)).toEqual({
          type: "u32",
          value: val,
          human: val.toString(),
        });
      }

      for (let i = 0; i < 100; i++) {
        const val = rng.nextInt(0, 0xffffffff);
        const encoded = encodeU32(val);
        expect(decodeScVal(encoded)).toBe(val);
        expect(decode(encoded)).toEqual({
          type: "u32",
          value: val,
          human: val.toString(),
        });
      }
    });

    it("correctly round-trips random signed 32-bit integers", () => {
      const boundaryI32s = [-2147483648, -1, 0, 1, 2147483647];
      for (const val of boundaryI32s) {
        const encoded = encodeI32(val);
        expect(decodeScVal(encoded)).toBe(val);
      }

      for (let i = 0; i < 100; i++) {
        const val = rng.nextInt(-2147483648, 2147483647);
        const encoded = encodeI32(val);
        expect(decodeScVal(encoded)).toBe(val);
      }
    });

    it("correctly round-trips random unsigned 64-bit integers without precision loss", () => {
      const boundaryU64s = [0n, 1n, 42n, 4294967295n, 4294967296n, 18446744073709551615n];
      for (const val of boundaryU64s) {
        const encoded = encodeU64(val);
        expect(decodeScVal(encoded)).toBe(val);
        expect(decode(encoded)).toEqual({
          type: "u64",
          value: val.toString(),
          human: val.toString(),
        });
      }

      for (let i = 0; i < 100; i++) {
        const val = rng.nextBigInt(64);
        const encoded = encodeU64(val);
        expect(decodeScVal(encoded)).toBe(val);
        expect(decode(encoded)).toEqual({
          type: "u64",
          value: val.toString(),
          human: val.toString(),
        });
      }
    });

    it("correctly round-trips random signed 64-bit integers", () => {
      const boundaryI64s = [-9223372036854775808n, -1n, 0n, 1n, 9223372036854775807n];
      for (const val of boundaryI64s) {
        const encoded = encodeI64(val);
        expect(decodeScVal(encoded)).toBe(val);
      }

      for (let i = 0; i < 100; i++) {
        const uval = rng.nextBigInt(64);
        const val = uval >= (1n << 63n) ? uval - (1n << 64n) : uval;
        const encoded = encodeI64(val);
        expect(decodeScVal(encoded)).toBe(val);
      }
    });

    it("correctly round-trips arbitrary strings and symbols", () => {
      for (let i = 0; i < 100; i++) {
        const str = rng.nextString(rng.nextInt(0, 40));
        const encoded = encodeSymbol(str);
        expect(decodeScVal(encoded)).toBe(str);
        expect(decode(encoded)).toEqual({
          type: "symbol",
          value: str,
          human: str,
        });
      }
    });

    it("correctly round-trips byte arrays to hex strings", () => {
      for (let i = 0; i < 50; i++) {
        const b = rng.nextBytes(rng.nextInt(0, 40));
        const expectedHex =
          "0x" +
          Array.from(b)
            .map((byte) => byte.toString(16).padStart(2, "0"))
            .join("");
        const encoded = encodeBytes(b);
        expect(decodeScVal(encoded)).toBe(expectedHex);
      }
    });

    it("correctly round-trips vectors of u32s", () => {
      for (let i = 0; i < 50; i++) {
        const len = rng.nextInt(0, 10);
        const vec: number[] = [];
        for (let j = 0; j < len; j++) {
          vec.push(rng.nextInt(0, 100000));
        }
        const encoded = encodeVecU32(vec);
        expect(decodeScVal(encoded)).toEqual(vec);
      }
    });
  });

  describe("Determinism property", () => {
    it("produces identical results across repeated decode calls", () => {
      for (let i = 0; i < 100; i++) {
        const val = rng.nextInt(0, 1000000);
        const encoded = encodeU32(val);
        const first = decode(encoded);
        const second = decode(encoded);
        expect(first).toEqual(second);
      }
    });
  });
});
