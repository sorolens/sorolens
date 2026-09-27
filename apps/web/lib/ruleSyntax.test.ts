import { describe, expect, it } from "vitest";
import { tokenizeRule, RULE_METRICS } from "./ruleSyntax";

/** Reassembling the tokens must reproduce the source exactly. */
function rebuilt(source: string) {
  return tokenizeRule(source)
    .map((t) => t.text)
    .join("");
}

describe("tokenizeRule", () => {
  it("round-trips the source text", () => {
    for (const source of [
      "error_rate > 5%",
      "fee_per_invocation > 0.5 XLM for 5m on contract CABC",
      "avg(cpu_insn_per_invocation) > 5000000 instructions for 30m on network testnet",
      "",
      "  ",
    ]) {
      expect(rebuilt(source)).toBe(source);
    }
  });

  it("classifies clauses, aggregations, metrics, numbers and units", () => {
    const tokens = tokenizeRule("avg(error_rate) > 0.5 XLM for 5m");
    const kinds = tokens
      .filter((t) => t.text.trim() !== "")
      .map((t) => [t.text, t.kind]);

    expect(kinds).toContainEqual(["avg", "aggregation"]);
    expect(kinds).toContainEqual(["error_rate", "metric"]);
    expect(kinds).toContainEqual([">", "operator"]);
    expect(kinds).toContainEqual(["0.5", "number"]);
    expect(kinds).toContainEqual(["XLM", "unit"]);
    expect(kinds).toContainEqual(["for", "keyword"]);
    // The duration suffix stays with the number.
    expect(kinds).toContainEqual(["5", "number"]);
    expect(kinds).toContainEqual(["m", "number"]);
  });

  it("knows every catalog metric", () => {
    for (const metric of RULE_METRICS) {
      const tokens = tokenizeRule(metric);
      expect(tokens).toHaveLength(1);
      expect(tokens[0].kind).toBe("metric");
    }
  });

  it("does not highlight a keyword used as part of a metric name", () => {
    const tokens = tokenizeRule("network_errors > 1");
    expect(tokens.find((t) => t.text === "network_errors")?.kind).toBe(
      "metric"
    );
  });

  it("accepts a custom metric set", () => {
    const tokens = tokenizeRule(
      "my_custom_metric > 1",
      new Set(["my_custom_metric"])
    );
    expect(tokens.find((t) => t.text === "my_custom_metric")?.kind).toBe(
      "metric"
    );
  });
});
