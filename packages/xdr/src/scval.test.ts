import { describe, it, expect } from "vitest";
import { decode, decodeWithSpec, encodeStrKey, type SpecType } from "./index";

// ── A minimal ScVal XDR encoder for building fixtures ───────────────────────

function u32(n: number): number[] {
  return [(n >>> 24) & 0xff, (n >>> 16) & 0xff, (n >>> 8) & 0xff, n & 0xff];
}

function uint(value: bigint, bytes: number): number[] {
  const width = BigInt(bytes * 8);
  let v = value < 0n ? (1n << width) + value : value;
  const out: number[] = [];
  for (let i = 0; i < bytes; i++) {
    out.unshift(Number(v & 0xffn));
    v >>= 8n;
  }
  return out;
}

function varBytes(data: number[]): number[] {
  const pad = (4 - (data.length % 4)) % 4;
  return [...u32(data.length), ...data, ...new Array(pad).fill(0)];
}

function text(s: string): number[] {
  return varBytes(Array.from(new TextEncoder().encode(s)));
}

const sc = {
  bool: (b: boolean) => [...u32(0), ...u32(b ? 1 : 0)],
  void: () => u32(1),
  error: (type: number, code: number) => [
    ...u32(2),
    ...u32(type),
    ...u32(code),
  ],
  u32: (n: number) => [...u32(3), ...u32(n)],
  i32: (n: number) => [...u32(4), ...u32(n >>> 0)],
  u64: (n: bigint) => [...u32(5), ...uint(n, 8)],
  i64: (n: bigint) => [...u32(6), ...uint(n, 8)],
  timepoint: (n: bigint) => [...u32(7), ...uint(n, 8)],
  duration: (n: bigint) => [...u32(8), ...uint(n, 8)],
  u128: (n: bigint) => [...u32(9), ...uint(n, 16)],
  i128: (n: bigint) => [...u32(10), ...uint(n, 16)],
  u256: (n: bigint) => [...u32(11), ...uint(n, 32)],
  i256: (n: bigint) => [...u32(12), ...uint(n, 32)],
  bytes: (data: number[]) => [...u32(13), ...varBytes(data)],
  string: (s: string) => [...u32(14), ...text(s)],
  symbol: (s: string) => [...u32(15), ...text(s)],
  vec: (items: number[][]) => [
    ...u32(16),
    ...u32(items.length),
    ...items.flat(),
  ],
  map: (entries: [number[], number[]][]) => [
    ...u32(17),
    ...u32(entries.length),
    ...entries.flatMap(([k, v]) => [...k, ...v]),
  ],
  account: (key: number[]) => [...u32(18), ...u32(0), ...u32(0), ...key],
  contract: (id: number[]) => [...u32(18), ...u32(1), ...id],
  muxed: (id: number[], key: number[]) => [
    ...u32(18),
    ...u32(2),
    ...u32(256),
    ...id,
    ...key,
  ],
  instance: (wasmHash: number[], storage: [number[], number[]][]) => [
    ...u32(19),
    ...u32(0),
    ...wasmHash,
    ...u32(1),
    ...u32(storage.length),
    ...storage.flatMap(([k, v]) => [...k, ...v]),
  ],
  ledgerKeyInstance: () => u32(20),
  nonce: (n: bigint) => [...u32(21), ...uint(n, 8)],
};

function b64(bytes: number[]): string {
  return btoa(String.fromCharCode(...bytes));
}

const ZERO32 = new Array(32).fill(0);

// ── Every ScVal type decodes to a stable { type, value, human } ─────────────

describe("decode - every ScVal type", () => {
  const cases: [string, number[], unknown, string][] = [
    ["bool", sc.bool(true), true, "true"],
    ["void", sc.void(), null, "void"],
    ["u32", sc.u32(42), 42, "42"],
    ["i32", sc.i32(-7), -7, "-7"],
    [
      "u64",
      sc.u64(18446744073709551615n),
      "18446744073709551615",
      "18446744073709551615",
    ],
    [
      "i64",
      sc.i64(-9223372036854775808n),
      "-9223372036854775808",
      "-9223372036854775808",
    ],
    [
      "u128",
      sc.u128((1n << 128n) - 1n),
      ((1n << 128n) - 1n).toString(),
      ((1n << 128n) - 1n).toString(),
    ],
    [
      "i128",
      sc.i128(-1000000000000000000000n),
      "-1000000000000000000000",
      "-1000000000000000000000",
    ],
    [
      "u256",
      sc.u256(1n << 200n),
      (1n << 200n).toString(),
      (1n << 200n).toString(),
    ],
    [
      "i256",
      sc.i256(-(1n << 255n)),
      (-(1n << 255n)).toString(),
      (-(1n << 255n)).toString(),
    ],
    [
      "bytes",
      sc.bytes([0xde, 0xad, 0xbe, 0xef, 0x01]),
      "0xdeadbeef01",
      "0xdeadbeef01",
    ],
    ["string", sc.string("hello world"), "hello world", '"hello world"'],
    ["symbol", sc.symbol("transfer"), "transfer", "transfer"],
  ];

  it.each(cases)("decodes %s", (type, bytes, value, human) => {
    expect(decode(b64(bytes))).toEqual({ type, value, human });
  });

  it("decodes timepoint as ISO time and duration in seconds", () => {
    expect(decode(b64(sc.timepoint(1767225600n)))).toEqual({
      type: "timepoint",
      value: "1767225600",
      human: "2026-01-01T00:00:00.000Z",
    });
    expect(decode(b64(sc.duration(3600n)))).toEqual({
      type: "duration",
      value: "3600",
      human: "3600s",
    });
  });

  it("decodes contract and host errors", () => {
    expect(decode(b64(sc.error(0, 7)))).toEqual({
      type: "error",
      value: { type: "Contract", code: 7 },
      human: "Error(Contract, #7)",
    });
    expect(decode(b64(sc.error(7, 5))).human).toBe(
      "Error(Budget, ExceededLimit)"
    );
  });

  it("decodes account and contract addresses as StrKeys", () => {
    expect(decode(b64(sc.account(ZERO32)))).toEqual({
      type: "address",
      value: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
      human: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
    });
    expect(decode(b64(sc.contract(ZERO32))).value).toBe(
      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"
    );
  });

  it("decodes a muxed account address as an M... StrKey", () => {
    const id = new Array(8).fill(0x22);
    const key = new Array(32).fill(0x33);
    const decoded = decode(b64(sc.muxed(id, key)));
    expect(decoded.type).toBe("address");
    // Recompute the expected M... independently: StrKey payload is the
    // 32-byte ed25519 key followed by the 8-byte uint64 id.
    const payload = new Uint8Array([...key, ...id]);
    const expected = encodeStrKey(12 << 3, payload);
    expect(decoded.value).toBe(expected);
  });

  it("decodes nested vecs and maps", () => {
    const decoded = decode(
      b64(sc.map([[sc.symbol("amounts"), sc.vec([sc.u32(1), sc.i128(-2n)])]]))
    );
    expect(decoded.type).toBe("map");
    expect(decoded.human).toBe("{amounts: [1, -2]}");
  });

  it("decodes contract instances and ledger keys", () => {
    const instance = decode(
      b64(sc.instance(ZERO32, [[sc.symbol("admin"), sc.bool(true)]]))
    );
    expect(instance.type).toBe("contract_instance");
    expect(instance.human).toBe(
      `ContractInstance(wasm:${"00".repeat(32)}, {admin: true})`
    );
    expect(decode(b64(sc.ledgerKeyInstance())).type).toBe(
      "ledger_key_contract_instance"
    );
    expect(decode(b64(sc.nonce(99n)))).toEqual({
      type: "ledger_key_nonce",
      value: "99",
      human: "Nonce(99)",
    });
  });

  it("returns the decode error for truncated or unknown input", () => {
    const error = { type: "error", value: null, human: "<decode_error>" };
    expect(decode(b64(sc.u128(1n).slice(0, 10)))).toEqual(error);
    expect(decode(b64(u32(99)))).toEqual(error);
  });
});

// ── Spec-aware decoding names struct fields, union cases and enum variants ──

const u32Type: SpecType = { kind: "primitive" };

const pointSpec: SpecType = {
  kind: "struct",
  name: "Point",
  fields: [
    { name: "x", type: u32Type },
    { name: "y", type: u32Type },
  ],
};

const statusSpec: SpecType = {
  kind: "enum",
  name: "Status",
  cases: [
    { name: "Pending", value: 0 },
    { name: "Active", value: 1 },
  ],
};

const actionSpec: SpecType = {
  kind: "union",
  name: "Action",
  cases: [{ name: "Stop" }, { name: "Move", types: [pointSpec] }],
};

describe("decodeWithSpec", () => {
  const point = sc.map([
    [sc.symbol("x"), sc.u32(1)],
    [sc.symbol("y"), sc.u32(2)],
  ]);

  it("names struct fields", () => {
    const decoded = decodeWithSpec(b64(point), pointSpec);
    expect(decoded.type).toBe("struct");
    expect(decoded.specName).toBe("Point");
    expect(decoded.human).toBe("Point { x: 1, y: 2 }");
  });

  it("names tuple-struct fields", () => {
    const spec: SpecType = {
      kind: "struct",
      name: "Pair",
      fields: [
        { name: "0", type: u32Type },
        { name: "1", type: u32Type },
      ],
    };
    expect(
      decodeWithSpec(b64(sc.vec([sc.u32(3), sc.u32(4)])), spec).human
    ).toBe("Pair(3, 4)");
  });

  it("names enum variants", () => {
    const decoded = decodeWithSpec(b64(sc.u32(1)), statusSpec);
    expect(decoded).toMatchObject({
      type: "enum",
      variant: "Active",
      human: "Status::Active",
    });
  });

  it("names union cases, recursing into their payload", () => {
    expect(
      decodeWithSpec(b64(sc.vec([sc.symbol("Stop")])), actionSpec).human
    ).toBe("Action::Stop");
    const moved = decodeWithSpec(
      b64(sc.vec([sc.symbol("Move"), point])),
      actionSpec
    );
    expect(moved.variant).toBe("Move");
    expect(moved.human).toBe("Action::Move(Point { x: 1, y: 2 })");
  });

  it("applies specs through vec, map, option and tuple types", () => {
    const vecSpec: SpecType = { kind: "vec", element: statusSpec };
    expect(
      decodeWithSpec(b64(sc.vec([sc.u32(0), sc.u32(1)])), vecSpec).human
    ).toBe("[Status::Pending, Status::Active]");
    const optionSpec: SpecType = { kind: "option", value: statusSpec };
    expect(decodeWithSpec(b64(sc.void()), optionSpec).human).toBe("void");
    expect(decodeWithSpec(b64(sc.u32(1)), optionSpec).human).toBe(
      "Status::Active"
    );
    const mapSpec: SpecType = { kind: "map", key: u32Type, value: statusSpec };
    expect(
      decodeWithSpec(b64(sc.map([[sc.symbol("s"), sc.u32(0)]])), mapSpec).human
    ).toBe("{s: Status::Pending}");
    const tupleSpec: SpecType = {
      kind: "tuple",
      elements: [u32Type, statusSpec],
    };
    expect(
      decodeWithSpec(b64(sc.vec([sc.u32(9), sc.u32(1)])), tupleSpec).human
    ).toBe("(9, Status::Active)");
  });

  it("falls back to plain decoding when the value does not match the spec", () => {
    const plain = decode(b64(sc.u32(5)));
    expect(decodeWithSpec(b64(sc.u32(5)), statusSpec)).toEqual(plain);
    expect(decodeWithSpec(b64(sc.u32(5)), pointSpec)).toEqual(plain);
    const missingField = sc.map([[sc.symbol("x"), sc.u32(1)]]);
    expect(decodeWithSpec(b64(missingField), pointSpec)).toEqual(
      decode(b64(missingField))
    );
  });
});
