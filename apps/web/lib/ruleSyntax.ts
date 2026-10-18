// Lightweight tokenizer for the Sorolens alert rule language
// (apps/api/rulelang). It powers the editor's syntax highlighting and the
// error-position gutter. It is intentionally permissive: unknown words are
// emitted as "metric" so a half-typed rule still highlights sensibly, and the
// server-side validator remains the source of truth.

export type RuleTokenKind =
  | "keyword"
  | "aggregation"
  | "metric"
  | "number"
  | "unit"
  | "operator"
  | "punctuation"
  | "plain";

export interface RuleToken {
  kind: RuleTokenKind;
  text: string;
}

/** Aggregation functions understood by the rule language. */
export const RULE_AGGREGATIONS = new Set([
  "avg",
  "max",
  "min",
  "sum",
  "rate",
  "count",
]);

/** Clause keywords. */
export const RULE_KEYWORDS = new Set(["for", "on", "contract", "network"]);

/** Unit words that may follow a number. */
export const RULE_UNITS = new Set([
  "xlm",
  "stroops",
  "instructions",
  "bytes",
  "ledgers",
  "%",
]);

/**
 * The metric catalog, mirrored from apps/api/rulelang/catalog.go. The editor
 * additionally renders the live catalog returned by
 * GET /api/v1/rules/metrics; this set only drives highlighting.
 */
export const RULE_METRICS = new Set([
  "invocations",
  "events",
  "failed_invocations",
  "error_rate",
  "uptime",
  "fee_per_invocation",
  "total_fee",
  "fee_per_invocation_stroops",
  "cpu_insn_per_invocation",
  "cpu_insn_total",
  "mem_byte_per_invocation",
  "ledger_read_bytes",
  "ledger_write_bytes",
  "storage_entries",
  "expiring_storage_entries",
  "min_storage_ttl_ledgers",
  "health_score",
]);

function isDigit(ch: string) {
  return ch >= "0" && ch <= "9";
}

function isLetter(ch: string) {
  return /[A-Za-z_]/.test(ch);
}

function isIdentPart(ch: string) {
  return /[A-Za-z0-9_]/.test(ch);
}

/**
 * tokenizeRule splits a rule source into highlighting tokens. Whitespace and
 * unrecognized characters are returned as "plain" so the editor can rebuild
 * the exact original text.
 */
export function tokenizeRule(
  source: string,
  metrics: Set<string> = RULE_METRICS
): RuleToken[] {
  const tokens: RuleToken[] = [];
  let i = 0;
  const push = (kind: RuleTokenKind, text: string) => {
    if (text) tokens.push({ kind, text });
  };

  while (i < source.length) {
    const ch = source[i];

    if (/\s/.test(ch)) {
      let j = i;
      while (j < source.length && /\s/.test(source[j])) j++;
      push("plain", source.slice(i, j));
      i = j;
      continue;
    }

    if (ch === '"' || ch === "'") {
      let j = i + 1;
      while (j < source.length && source[j] !== ch) j++;
      if (j < source.length) j++;
      push("plain", source.slice(i, j));
      i = j;
      continue;
    }

    if (isDigit(ch) || ch === ".") {
      let j = i;
      while (j < source.length && /[0-9.]/.test(source[j])) j++;
      // scientific notation, e.g. 5e6
      if (j < source.length && (source[j] === "e" || source[j] === "E")) {
        const next = source[j + 1];
        if (
          isDigit(next) ||
          ((next === "+" || next === "-") && isDigit(source[j + 2]))
        ) {
          j += 1;
          if (source[j] === "+" || source[j] === "-") j += 1;
          while (j < source.length && isDigit(source[j])) j++;
        }
      }
      push("number", source.slice(i, j));
      // duration suffix (5m) stays part of the number token.
      if (j < source.length && /[a-zA-Z]/.test(source[j])) {
        let k = j;
        while (k < source.length && /[a-zA-Z]/.test(source[k])) k++;
        push("number", source.slice(j, k));
        i = k;
        continue;
      }
      i = j;
      continue;
    }

    if (isLetter(ch)) {
      let j = i;
      while (j < source.length && isIdentPart(source[j])) j++;
      const word = source.slice(i, j);
      const lower = word.toLowerCase();
      let kind: RuleTokenKind = "metric";
      if (RULE_KEYWORDS.has(lower)) kind = "keyword";
      else if (RULE_AGGREGATIONS.has(lower)) kind = "aggregation";
      else if (RULE_UNITS.has(lower)) kind = "unit";
      else if (!metrics.has(lower)) kind = "metric";
      push(kind, word);
      i = j;
      continue;
    }

    if (ch === "%") {
      push("unit", ch);
      i++;
      continue;
    }

    if ("<>=".includes(ch) || ch === "!") {
      let j = i;
      if (ch === ">" && source[i + 1] === "=") j = i + 2;
      else if (ch === "<" && source[i + 1] === "=") j = i + 2;
      else if (ch === "=" && source[i + 1] === "=") j = i + 2;
      else if (ch === "!" && source[i + 1] === "=") j = i + 2;
      else j = i + 1;
      push("operator", source.slice(i, j));
      i = j;
      continue;
    }

    if (ch === "(" || ch === ")") {
      push("punctuation", ch);
      i++;
      continue;
    }

    push("plain", ch);
    i++;
  }

  return tokens;
}

/** classForToken maps a token kind to its Tailwind color classes. */
export function classForToken(kind: RuleTokenKind): string {
  switch (kind) {
    case "keyword":
      return "text-fuchsia-400";
    case "aggregation":
      return "text-sky-400";
    case "metric":
      return "text-emerald-400";
    case "number":
      return "text-amber-400";
    case "unit":
      return "text-amber-300";
    case "operator":
      return "text-rose-400";
    case "punctuation":
      return "text-[var(--color-text-secondary)]";
    default:
      return "text-[var(--color-text-primary)]";
  }
}
