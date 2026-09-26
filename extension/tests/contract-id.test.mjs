import test from "node:test";
import assert from "node:assert/strict";

import {
  contractIdFromUrl,
  findContractIds,
  isContractId,
  normalizeContractId,
  shortenContractId,
} from "../src/shared/contract-id.js";

const VALID = "C" + "A".repeat(55);
const OTHER = "C" + "B".repeat(55);

test("isContractId accepts a 56-char base32 StrKey", () => {
  assert.equal(isContractId(VALID), true);
  assert.equal(VALID.length, 56);
});

test("isContractId rejects wrong length, prefix and alphabet", () => {
  assert.equal(isContractId(""), false);
  assert.equal(isContractId(VALID.slice(1)), false);
  assert.equal(isContractId(VALID + "A"), false);
  assert.equal(isContractId("G" + "A".repeat(55)), false);
  assert.equal(isContractId("C" + "a".repeat(55)), false);
  // 0, 1, 8 and 9 are not in the base32 alphabet.
  assert.equal(isContractId("C" + "0".repeat(55)), false);
  assert.equal(isContractId("C" + "1".repeat(55)), false);
  assert.equal(isContractId("C" + "8".repeat(54) + "A"), false);
  assert.equal(isContractId("C" + "9".repeat(54) + "A"), false);
  assert.equal(isContractId(null), false);
  assert.equal(isContractId(42), false);
});

test("findContractIds returns distinct IDs in order", () => {
  const text = `tracked ${VALID} then ${OTHER} then ${VALID} again`;
  assert.deepEqual(findContractIds(text), [VALID, OTHER]);
});

test("findContractIds ignores IDs embedded in longer base32 tokens", () => {
  assert.deepEqual(findContractIds("X" + VALID), []);
  assert.deepEqual(findContractIds(VALID + "A"), []);
  // Surrounded by punctuation is fine.
  assert.deepEqual(findContractIds(`"${VALID}".`), [VALID]);
});

test("findContractIds handles non-string and short input", () => {
  assert.deepEqual(findContractIds(undefined), []);
  assert.deepEqual(findContractIds(""), []);
  assert.deepEqual(findContractIds("CAAAA"), []);
});

test("normalizeContractId trims, strips zero-width chars and upper-cases", () => {
  assert.equal(normalizeContractId(`  ${VALID}\n`), VALID);
  assert.equal(normalizeContractId(`${VALID.slice(0, 10)}\u200b${VALID.slice(10)}`), VALID);
  assert.equal(normalizeContractId(VALID.toLowerCase()), VALID);
  assert.equal(normalizeContractId("not-a-contract"), null);
});

test("contractIdFromUrl reads explorer URLs", () => {
  assert.equal(
    contractIdFromUrl(`https://stellar.expert/explorer/public/contract/${VALID}`),
    VALID,
  );
  assert.equal(
    contractIdFromUrl(`https://lab.stellar.org/r/testnet/contract/${OTHER}`),
    OTHER,
  );
  assert.equal(contractIdFromUrl("https://stellar.org"), null);
  assert.equal(contractIdFromUrl(""), null);
});

test("shortenContractId keeps the head and tail", () => {
  assert.equal(shortenContractId(VALID), `${VALID.slice(0, 6)}…${VALID.slice(-4)}`);
  assert.equal(shortenContractId("short"), "short");
  assert.equal(shortenContractId(undefined), "");
});
