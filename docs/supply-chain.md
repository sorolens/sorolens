# Supply chain scans

The Supply chain workflow runs on Tuesday at 04:23 UTC and can also be run from
the Actions tab. It checks Go modules, Cargo lockfiles, Python SDK dependencies,
and production Node dependencies. The Node jobs fail on high and critical
advisories. Go, Rust, and Python fail on any vulnerability their scanners
report. Investigate scanner errors as well as advisory findings.

Gitleaks scans the checked-out Git history on the same schedule, on manual runs,
and on pushes to main. The fixture exceptions in [.gitleaksignore](https://github.com/sorolens/sorolens/blob/main/.gitleaksignore)
are the only scan exceptions. Add narrow fixture exceptions with a reason.
Dependency advisory exceptions are currently empty.

Tagged releases include an SPDX JSON SBOM for each published API and indexer
image. The release workflow extracts each SBOM from the immutable image digest,
attaches it to the GitHub Release, and includes its hash in checksums.txt.
