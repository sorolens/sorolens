//! On-chain monitoring policies stored by the watchdog contract.
//!
//! The watchdog contract (`contracts/watchdog`) keeps one bounded
//! [`MonitoringPolicy`] per monitored contract. This module is read-only: it
//! parses the JSON printed by `stellar contract invoke ... -- get_policy`
//! (or `sorolens watchdog policy get`) and mirrors the contract's validation
//! bounds so callers can check a policy before submitting it.

use serde::{Deserialize, Serialize};

/// Largest accepted `expected_interval` in seconds (one year).
pub const MAX_POLICY_INTERVAL: u64 = 31_536_000;
/// Largest accepted alert threshold per monitoring window.
pub const MAX_POLICY_THRESHOLD: u32 = 1_000_000;

/// Monitoring policy for one contract, as stored on-chain.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MonitoringPolicy {
    /// Alerts per window at which the contract is degraded (at least 1).
    pub warning_threshold: u32,
    /// Alerts per window at which the contract is critical (>= warning).
    pub critical_threshold: u32,
    /// Expected seconds between status reports.
    pub expected_interval: u64,
    /// Operator the policy belongs to (a Stellar address).
    pub owner: String,
}

impl MonitoringPolicy {
    /// Parses the JSON output of `get_policy`; `null` means no policy is set.
    pub fn from_json(json: &str) -> Result<Option<Self>, serde_json::Error> {
        serde_json::from_str(json)
    }

    /// Checks the same bounds the contract enforces.
    pub fn validate(&self) -> Result<(), String> {
        if self.warning_threshold == 0 {
            return Err("warning_threshold must be at least 1".into());
        }
        if self.warning_threshold > self.critical_threshold {
            return Err("warning_threshold must not exceed critical_threshold".into());
        }
        if self.critical_threshold > MAX_POLICY_THRESHOLD {
            return Err(format!("critical_threshold exceeds {MAX_POLICY_THRESHOLD}"));
        }
        if self.expected_interval == 0 || self.expected_interval > MAX_POLICY_INTERVAL {
            return Err(format!(
                "expected_interval must be within 1..={MAX_POLICY_INTERVAL}"
            ));
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const JSON: &str =
        r#"{"warning_threshold":3,"critical_threshold":10,"expected_interval":300,"owner":"GABC"}"#;

    #[test]
    fn parses_and_validates() {
        let p = MonitoringPolicy::from_json(JSON).unwrap().unwrap();
        assert_eq!(p.critical_threshold, 10);
        assert!(p.validate().is_ok());
    }

    #[test]
    fn null_means_no_policy() {
        assert_eq!(MonitoringPolicy::from_json("null").unwrap(), None);
    }

    #[test]
    fn rejects_out_of_bounds() {
        let mut p = MonitoringPolicy::from_json(JSON).unwrap().unwrap();
        p.warning_threshold = 11;
        assert!(p.validate().is_err());
        p.warning_threshold = 0;
        assert!(p.validate().is_err());
        p.warning_threshold = 1;
        p.expected_interval = MAX_POLICY_INTERVAL + 1;
        assert!(p.validate().is_err());
    }
}
