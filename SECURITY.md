# Security Policy
## Reporting a vulnerability
If you believe you have found a security vulnerability in Sorolens, please report it by email:
**security@sorolens.dev**
Do not open a public GitHub issue for security vulnerabilities. Public disclosure before a fix is available puts all users of the project at risk.
### What to include in your report
- A description of the vulnerability and the component it affects (API, indexer, dashboard, CLI, XDR decoder, or fixture contract).
- Steps to reproduce the issue, including any relevant contract IDs, network (testnet or mainnet), or request payloads.
- Your assessment of the potential impact.
- Whether you have already developed a proof of concept (you do not need to share exploit code, a description is sufficient).
### What happens after you report
| Timeline | What we do |
|---|---|
| 48 hours | Acknowledge receipt of your report by email. |
| 7 days | Provide an initial assessment: whether we can reproduce the issue and a preliminary severity rating. |
| 90 days | Publish a fix and disclose the vulnerability publicly (CVE if applicable). If a fix requires more time, we will communicate that before the 90-day mark and agree on an extended timeline with you. |
We ask that you do not disclose the vulnerability publicly before the 90-day window has elapsed or before we have published a fix, whichever comes first. We will credit you by name (or by handle if you prefer) in the release notes when we publish the fix, unless you ask to remain anonymous.
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
## Supported versions
Only the latest release receives security fixes. If you are running an older version, please upgrade before reporting.
