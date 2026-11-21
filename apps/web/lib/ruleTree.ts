import { RULE_AGGREGATIONS, RULE_METRICS, RULE_UNITS } from "./ruleSyntax";

export interface VisualRuleTree {
  aggregation?: string;
  metric: string;
  operator: string;
  threshold: number;
  unit?: string;
  duration?: string;
  contractId?: string;
  network?: string;
}

export const SUPPORTED_OPERATORS = [">", ">=", "<", "<=", "==", "!="] as const;

export const DEFAULT_RULE_TREE: VisualRuleTree = {
  metric: "fee_per_invocation",
  operator: ">",
  threshold: 0.5,
  unit: "XLM",
  duration: "5m",
};

/**
 * parseDslToTree parses a Sorolens alert rule DSL string into a structured VisualRuleTree.
 */
export function parseDslToTree(dsl: string): VisualRuleTree {
  const trimmed = dsl.trim();
  if (!trimmed) {
    return { ...DEFAULT_RULE_TREE };
  }

  const result: VisualRuleTree = {
    metric: "invocations",
    operator: ">",
    threshold: 0,
  };

  let remainder = trimmed;

  // Extract 'on network <name>'
  const networkMatch = remainder.match(/\bon\s+network\s+([A-Za-z0-9_-]+)/i);
  if (networkMatch) {
    result.network = networkMatch[1];
    remainder = remainder.replace(networkMatch[0], "").trim();
  }

  // Extract 'on contract <id>'
  const contractMatch = remainder.match(/\bon\s+contract\s+([A-Za-z0-9_"-]+)/i);
  if (contractMatch) {
    result.contractId = contractMatch[1].replace(/["']/g, "");
    remainder = remainder.replace(contractMatch[0], "").trim();
  }

  // Extract 'for <duration>'
  const forMatch = remainder.match(/\bfor\s+([0-9]+[smhd])/i);
  if (forMatch) {
    result.duration = forMatch[1];
    remainder = remainder.replace(forMatch[0], "").trim();
  }

  // Find operator (>=, <=, ==, !=, >, <)
  const opMatch = remainder.match(/(>=|<=|==|!=|>|<)/);
  if (!opMatch || opMatch.index === undefined) {
    return {
      ...DEFAULT_RULE_TREE,
      metric: remainder.trim() || DEFAULT_RULE_TREE.metric,
    };
  }

  result.operator = opMatch[1];
  const left = remainder.slice(0, opMatch.index).trim();
  const right = remainder.slice(opMatch.index + opMatch[1].length).trim();

  // Parse left side: bare metric or aggregation(metric)
  const aggMatch = left.match(/^([a-zA-Z_]+)\(([^)]+)\)$/);
  if (aggMatch) {
    const fn = aggMatch[1].toLowerCase();
    if (RULE_AGGREGATIONS.has(fn)) {
      result.aggregation = fn;
      result.metric = aggMatch[2].trim();
    } else {
      result.metric = left;
    }
  } else {
    result.metric = left;
  }

  // Parse right side: number, optional unit / percentage
  const numMatch = right.match(
    /^([+-]?[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\s*(%|[a-zA-Z]+)?/
  );
  if (numMatch) {
    result.threshold = parseFloat(numMatch[1]);
    if (numMatch[2]) {
      result.unit = numMatch[2];
    }
  }

  return result;
}

/**
 * treeToDsl formats a VisualRuleTree into a valid Sorolens alert rule DSL string.
 */
export function treeToDsl(tree: VisualRuleTree): string {
  const left = tree.aggregation
    ? `${tree.aggregation}(${tree.metric})`
    : tree.metric;
  let valStr = String(tree.threshold);
  if (tree.unit) {
    if (tree.unit === "%") {
      valStr = `${tree.threshold}%`;
    } else {
      valStr = `${tree.threshold} ${tree.unit}`;
    }
  }

  let dsl = `${left} ${tree.operator} ${valStr}`;
  if (tree.duration) {
    dsl += ` for ${tree.duration}`;
  }
  if (tree.contractId) {
    dsl += ` on contract ${tree.contractId}`;
  }
  if (tree.network) {
    dsl += ` on network ${tree.network}`;
  }

  return dsl;
}
