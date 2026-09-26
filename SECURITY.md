# Security Policy

Sorolens takes the security of its API, indexer, dashboard, CLI, and on-chain contracts seriously. This document explains how to report a vulnerability, what to expect from us in return, and how coordinated disclosure works.

## Table of contents

- [Supported versions](#supported-versions)
- [Reporting a vulnerability](#reporting-a-vulnerability)
- [What to include in your report](#what-to-include-in-your-report)
- [Response timeline (SLA)](#response-timeline-sla)
- [Coordinated disclosure](#coordinated-disclosure)
- [Researcher and maintainer responsibilities](#researcher-and-maintainer-responsibilities)
- [Bounty program](#bounty-program)
- [Scope](#scope)
- [Questions](#questions)

## Supported versions

Only the latest release receives security fixes. If you are running an older version, please upgrade before reporting.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

If you believe you have found a security vulnerability in Sorolens, please report it by email:

**security@sorolens.dev**

Do not open a public GitHub issue for security vulnerabilities. Public disclosure before a fix is available puts all users of the project at risk.

Email is currently the only confidential reporting channel: GitHub private vulnerability reporting is not enabled for this repository.

### What to include in your report

- A description of the vulnerability and the component it affects (API, indexer, dashboard, CLI, XDR decoder, or fixture contract).
- Steps to reproduce the issue, including any relevant contract IDs, network (testnet or mainnet), or request payloads.
- Your assessment of the potential impact.
- Whether you have already developed a proof of concept (you do not need to share exploit code, a description is sufficient).

## Response timeline (SLA)

| Timeline | What we do |
| --- | --- |
| 48 hours | Acknowledge receipt of your report by email. |
| 7 days | Provide an initial assessment: whether we can reproduce the issue and a preliminary severity rating. |
| 90 days | Publish a fix and disclose the vulnerability publicly (CVE if applicable). |

If a fix requires more time, we will communicate that before the 90-day mark and agree on an extended timeline with you.

## Coordinated disclosure

We follow a coordinated disclosure model:

1. **Report privately.** Send the details to security@sorolens.dev instead of opening a public issue.
2. **Acknowledge and triage.** We confirm receipt within 48 hours and share a preliminary assessment within 7 days.
3. **Fix and agree a date.** We develop and test a fix and agree a disclosure date with you. Our target is 90 days from the initial report.
4. **Publish.** We release the fix and publish the disclosure (with a CVE where applicable) once the fix is available.
5. **Credit.** We credit you by name (or by handle if you prefer) in the release notes unless you ask to remain anonymous.

We ask that you do not disclose the vulnerability publicly before the 90-day window has elapsed or before we have published a fix, whichever comes first.

## Researcher and maintainer responsibilities

Researchers:

- Report privately and give us a reasonable opportunity to fix the issue before any public disclosure.
- Avoid privacy violations, data destruction, and service disruption while testing.
- Only interact with accounts, data, and systems you own or have explicit permission to test.

Maintainers:

- Acknowledge reports within 48 hours and keep the reporter informed of progress.
- Confirm the affected versions, the severity, and the remediation.
- Credit the reporter in the release notes unless they ask to remain anonymous.

## Bounty program

There is no bug bounty program at this time.

## Scope

This policy covers:

- The Sorolens API (`apps/api`)
- The Sorolens indexer (`services/indexer`)
- The Sorolens CLI (`cli/`)
- The XDR decoder package (`packages/xdr`)
- The Sorolens dashboard (`apps/web`)
- The fixture contract (`contracts/counter`) when deployed to testnet or mainnet

Out of scope:

- Vulnerabilities in third-party dependencies (please report those to the upstream project). You are welcome to notify us as well so we can track the upstream fix.
- The Stellar network or Soroban protocol itself.
- Infrastructure operated by Neon, Upstash, or Vercel.

## Questions

For anything unclear about this policy, email security@sorolens.dev.
