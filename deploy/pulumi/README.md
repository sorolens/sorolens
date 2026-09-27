# @sorolens/pulumi

Pulumi (TypeScript) components that provision a self-hosted Sorolens stack.
They mirror the [Terraform modules](../terraform): same resources, same
inputs (camelCase instead of snake_case), same secure defaults.

Status: **AWS only** (`SorolensAws`). The package is `private` and not
published to npm yet; publishing is a later phase of #274.

## Usage

```ts
import { SorolensAws } from "@sorolens/pulumi";

const sorolens = new SorolensAws("sorolens", {
  apiImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-api:0.1.0",
  indexerImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-indexer:0.1.0",
  dashboardImage: "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-dashboard:0.1.0",
  certificateArn: "arn:aws:acm:us-east-1:123456789012:certificate/...",
  domainName: "sorolens.example.com",
  route53ZoneId: "Z0000000000000000000",
});

export const url = sorolens.url;
```

The region is the AWS provider's (`pulumi config set aws:region us-east-1`).
[examples/aws](examples/aws) is a complete program driven by stack config;
the [AWS install guide](../../docs/self-hosting/aws.md) walks through it.

Invalid input throws a `SorolensConfigError` before any resource is
registered. The same caveats as the Terraform module apply: images are not
published yet (#96 / #381), the indexer runs as exactly one task and is still
stubbed on `main`, migrations depend on #381, and the dashboard's
`NEXT_PUBLIC_API_URL` is baked in at build time.

## Layout

```
src/core/config.ts        cloud-agnostic settings (mirrors terraform modules/core)
src/aws/sorolens-aws.ts   SorolensAws component resource
examples/aws/             Pulumi program using the component
scripts/offline-preview.sh  pulumi preview without an AWS account (used by CI)
test/                     unit tests with Pulumi runtime mocks
```

## Development

This directory is its own pnpm workspace (its dependencies stay out of the
repository's root lockfile). Node 22+.

```sh
pnpm install
pnpm typecheck
pnpm build
pnpm test                          # Pulumi runtime mocks: no credentials, no backend
./scripts/offline-preview.sh https  # needs the pulumi CLI; fake credentials, local state
./scripts/offline-preview.sh http
```

`offline-preview.sh` never creates anything: `pulumi preview` runs against a
throwaway local backend with fake AWS credentials and the provider's
credential/account/metadata checks disabled.
