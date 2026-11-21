export const HEALTH_COMPONENTS = [
  {
    key: "uptime",
    label: "Uptime",
    weight: 40,
    explanation: "How often recent watchdog checks found the contract healthy.",
  },
  {
    key: "error_rate",
    label: "Error rate",
    weight: 25,
    explanation: "A higher score means fewer recent invocations failed.",
  },
  {
    key: "performance",
    label: "Performance",
    weight: 20,
    explanation:
      "Compares recent cost per invocation with the earlier baseline.",
  },
  {
    key: "storage_ttl",
    label: "Storage TTL",
    weight: 15,
    explanation:
      "Measures how much live storage has enough time before expiry.",
  },
] as const;
