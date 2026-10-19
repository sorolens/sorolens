# Maintainer Guide

This document describes the maintainer process and responsibilities for the Sorolens repository. It sets expectations for contributors and helps keep review and releases predictable.

This guide complements [CONTRIBUTING.md](CONTRIBUTING.md) and [ARCHITECTURE.md](ARCHITECTURE.md).

## Who is a maintainer

Maintainers are contributors with write access to the repository who are responsible for triaging issues, reviewing pull requests, and shipping releases. The current list of maintainers is visible in the repository settings and in the codeowners configuration if present.

## Responsibilities

- Triage new issues within a reasonable timeframe (generally a week).
- Label issues according to type, area, and difficulty.
- Review pull requests for correctness, tests, and adherence to the project style.
- Ensure CI is green before merging.
- Maintain the roadmap and documentation as the project evolves.
- Cut releases and keep the changelog up to date.
- Moderate discussions and enforce the code of conduct.

## Pull request process

1. A contributor opens a pull request against `main` and links the relevant issue using `Closes #<number>`.
2. A maintainer checks that the scope matches the issue and that tests and linting pass.
3. Review is performed with a focus on correctness, security, and maintainability.
4. Once CIS is green and at least one maintainer has approved, the PR is merged.
5. Large or risky changes may require additional review from another maintainer.

## Issue triage

- Assign issues to contributors who comment to claim them.
- Apply labels for type (bug, feature, docs), area (decoder, api, web, infra), and difficulty.
- Close stale issues or issues that are out of scope with a clear explanation.
- Use the issue templates in `.github/ISSUE_TEMPLATE/` to keep reports consistent.

## Release process

1. Ensure `main` is green and all intended changes are merged.
2. Update the version and changelog following the existing conventions.
3. Tag the release and publish the release notes.
4. Announce the release in the project Discord and on the website.

## Becoming a maintainer

Consistent contributors who demonstrate good judgment, reliable reviews, and a constructive attitude may be invited to become maintainers. If you are interested, reach out on Discord or open a discussion issue.

## Related documents

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [ROADMAP.md](ROADMAP.md)
- [docs/openapi.yaml](docs/openapi.yaml)
