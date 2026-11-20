import { describe, expect, it } from "vitest";
import { parseDslToTree, treeToDsl, type VisualRuleTree } from "./ruleTree";

describe("ruleTree round-tripping", () => {
  it("parses and serializes simple metric comparison", () => {
    const dsl = "fee_per_invocation > 0.5 XLM for 5m";
    const tree = parseDslToTree(dsl);
    expect(tree).toEqual({
      metric: "fee_per_invocation",
      operator: ">",
      threshold: 0.5,
      unit: "XLM",
      duration: "5m",
    });
    expect(treeToDsl(tree)).toBe(dsl);
  });

  it("parses and serializes aggregation functions", () => {
    const dsl = "avg(cpu_insn_per_invocation) <= 1000000 for 10m";
    const tree = parseDslToTree(dsl);
    expect(tree.aggregation).toBe("avg");
    expect(tree.metric).toBe("cpu_insn_per_invocation");
    expect(tree.operator).toBe("<=");
    expect(tree.threshold).toBe(1000000);
    expect(tree.duration).toBe("10m");
    expect(treeToDsl(tree)).toBe(dsl);
  });

  it("handles percentage units", () => {
    const dsl = "error_rate > 5% for 15m";
    const tree = parseDslToTree(dsl);
    expect(tree.metric).toBe("error_rate");
    expect(tree.operator).toBe(">");
    expect(tree.threshold).toBe(5);
    expect(tree.unit).toBe("%");
    expect(treeToDsl(tree)).toBe(dsl);
  });

  it("handles on contract and on network clauses", () => {
    const dsl = "invocations >= 100 for 1m on contract CDLZFC3 on network testnet";
    const tree = parseDslToTree(dsl);
    expect(tree.metric).toBe("invocations");
    expect(tree.operator).toBe(">=");
    expect(tree.threshold).toBe(100);
    expect(tree.duration).toBe("1m");
    expect(tree.contractId).toBe("CDLZFC3");
    expect(tree.network).toBe("testnet");
    expect(treeToDsl(tree)).toBe(dsl);
  });

  it("losslessly round-trips an arbitrary VisualRuleTree", () => {
    const original: VisualRuleTree = {
      aggregation: "max",
      metric: "ledger_read_bytes",
      operator: ">=",
      threshold: 65536,
      unit: "bytes",
      duration: "30m",
      network: "mainnet",
    };
    const dsl = treeToDsl(original);
    const roundTripped = parseDslToTree(dsl);
    expect(roundTripped).toEqual(original);
  });
});
