import { beforeAll, describe, expect, it } from "vitest";

import { SorolensAws, type SorolensAwsArgs } from "../src/aws/sorolens-aws";
import { SorolensConfigError } from "../src/core/config";
import { installMocks, valueOf } from "./mocks";

// The component only registers resources in its constructor, so importing it
// before the mocks are installed is safe; every stack is built after this.
beforeAll(async () => {
  await installMocks();
});

const images = {
  apiImage: "ghcr.io/example/sorolens-api:0.1.0",
  indexerImage: "ghcr.io/example/sorolens-indexer:0.1.0",
  dashboardImage: "ghcr.io/example/sorolens-dashboard:0.1.0",
};

let seq = 0;
function stack(args: Partial<SorolensAwsArgs> = {}): SorolensAws {
  return new SorolensAws(`t${++seq}`, { ...images, ...args });
}

interface ContainerDefinition {
  command?: string[];
  image: string;
  environment: { name: string; value: string }[];
  secrets: { name: string; valueFrom: string }[];
}

async function container(s: SorolensAws, key: "api" | "dashboard" | "indexer" | "migrations"): Promise<ContainerDefinition> {
  const json = await valueOf(s.taskDefinitions[key].containerDefinitions);
  return (JSON.parse(json) as ContainerDefinition[])[0]!;
}

const names = (items: { name: string }[]) => items.map((i) => i.name);

describe("SorolensAws", () => {
  it("requires HTTPS or an explicit HTTP opt-in", () => {
    expect(() => stack()).toThrow(/certificateArn/);
  });

  describe("secure defaults (HTTP evaluation mode)", () => {
    let s: SorolensAws;
    beforeAll(() => {
      s = stack({ allowHttpOnly: true });
    });

    it("keeps Aurora encrypted, private and protected", async () => {
      expect(await valueOf(s.database.engine)).toBe("aurora-postgresql");
      expect(await valueOf(s.database.engineVersion)).toMatch(/^16/);
      expect(await valueOf(s.database.storageEncrypted)).toBe(true);
      expect(await valueOf(s.database.deletionProtection)).toBe(true);
      expect(await valueOf(s.database.skipFinalSnapshot)).toBe(false);
      for (const i of s.databaseInstances) {
        expect(await valueOf(i.publiclyAccessible)).toBe(false);
      }
    });

    it("encrypts Redis at rest and in transit", async () => {
      expect(await valueOf(s.redis.atRestEncryptionEnabled)).toBe(true);
      expect(await valueOf(s.redis.transitEncryptionEnabled)).toBe(true);
    });

    it("only lets Sorolens tasks reach PostgreSQL and Redis", async () => {
      for (const [rules, port] of [[s.dbIngressRules, 5432], [s.redisIngressRules, 6379]] as const) {
        expect(rules).toHaveLength(2);
        for (const r of rules) {
          expect(await valueOf(r.cidrIpv4)).toBeUndefined();
          expect(await valueOf(r.referencedSecurityGroupId)).toBeTruthy();
          expect(await valueOf(r.fromPort)).toBe(port);
        }
      }
    });

    it("runs tasks privately without ECS Exec", async () => {
      for (const svc of Object.values(s.services)) {
        const net = await valueOf(svc.networkConfiguration);
        expect(net?.assignPublicIp).toBe(false);
        expect(await valueOf(svc.enableExecuteCommand)).toBe(false);
      }
    });

    it("runs exactly one indexer, never two at once", async () => {
      expect(await valueOf(s.services.indexer.desiredCount)).toBe(1);
      expect(await valueOf(s.services.indexer.deploymentMaximumPercent)).toBe(100);
      expect(await valueOf(s.services.indexer.deploymentMinimumHealthyPercent)).toBe(0);
      expect((await container(s, "indexer")).command).toEqual(["-mode=continuous", "-poll-interval=5m"]);
    });

    it("injects connection URLs only as secrets", async () => {
      for (const key of ["api", "indexer", "migrations"] as const) {
        const c = await container(s, key);
        expect(names(c.secrets)).toEqual(expect.arrayContaining(["DATABASE_URL", "REDIS_URL"]));
      }
      for (const key of ["api", "dashboard", "indexer", "migrations"] as const) {
        const c = await container(s, key);
        for (const secret of ["DATABASE_URL", "DIRECT_DATABASE_URL", "REDIS_URL"]) {
          expect(names(c.environment)).not.toContain(secret);
        }
        expect(JSON.stringify(c)).not.toContain("MockGeneratedPassword");
      }
      expect((await container(s, "dashboard")).secrets).toEqual([]);
    });

    it("stores TLS connection URLs in Secrets Manager", async () => {
      expect(await valueOf(s.databaseUrlSecretVersion.secretString)).toMatch(/^postgres:\/\/sorolens:.+\?sslmode=require$/);
      expect(await valueOf(s.redisUrlSecretVersion.secretString)).toMatch(/^rediss:\/\//);
    });

    it("grants the execution role only its own secrets and the task role nothing", async () => {
      const policy = JSON.parse(await valueOf(s.executionRolePolicy.policy)) as { Statement: { Sid: string; Resource: string | string[] }[] };
      const secrets = policy.Statement.find((st) => st.Sid === "ReadReferencedSecrets")!;
      expect(secrets.Resource).toEqual([await valueOf(s.databaseUrlSecretArn), await valueOf(s.redisUrlSecretArn)]);
      expect(secrets.Resource).not.toContain("*");
      expect(await valueOf(s.taskRole.inlinePolicies)).toBeUndefined();
      expect(await valueOf(s.taskRole.managedPolicyArns)).toBeUndefined();
    });

    it("serves plain HTTP on the load balancer only because it was opted in", async () => {
      expect(s.httpListener).toBeDefined();
      expect(s.httpsListener).toBeUndefined();
      expect(s.httpRedirectListener).toBeUndefined();
      expect(await valueOf(s.url)).toBe("http://sorolens-alb-123456.us-east-1.elb.amazonaws.com");
      expect(await valueOf(s.loadBalancer.dropInvalidHeaderFields)).toBe(true);
    });

    it("uses the first two zones and a single NAT gateway", async () => {
      expect(await Promise.all(s.privateSubnets.map((sub) => valueOf(sub.availabilityZone)))).toEqual(["us-east-1a", "us-east-1b"]);
      expect(await Promise.all(s.privateSubnets.map((sub) => valueOf(sub.cidrBlock)))).toEqual(["10.40.128.0/20", "10.40.144.0/20"]);
      expect(s.natGateways).toHaveLength(1);
    });

    it("keeps the image's own command for migrations by default", async () => {
      const c = await container(s, "migrations");
      expect(c.command).toBeUndefined();
      expect(c.image).toBe(images.apiImage);
    });
  });

  it("serves HTTPS with a redirect and an alias record when a domain is given", async () => {
    const s = stack({
      certificateArn: "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000",
      domainName: "sorolens.example.com",
      route53ZoneId: "Z0000000000000000000",
    });
    expect(s.httpListener).toBeUndefined();
    expect(await valueOf(s.httpsListener!.sslPolicy)).toBe("ELBSecurityPolicy-TLS13-1-2-2021-06");
    const redirect = await valueOf(s.httpRedirectListener!.defaultActions);
    expect(redirect[0]?.redirect?.protocol).toBe("HTTPS");
    expect(await valueOf(s.dnsRecord!.name)).toBe("sorolens.example.com");
    expect(await valueOf(s.url)).toBe("https://sorolens.example.com");
    const env = (await container(s, "dashboard")).environment;
    expect(env.find((e) => e.name === "NEXT_PUBLIC_API_URL")?.value).toBe("https://sorolens.example.com");
  });

  it("wires extra secrets, the migrations command and explicit topology", async () => {
    const sentry = "arn:aws:secretsmanager:us-east-1:123456789012:secret:sentry-dsn";
    const s = stack({
      allowHttpOnly: true,
      apiExtraSecrets: { SENTRY_DSN: sentry },
      migrationsCommand: ["/app/migrate", "up"],
      availabilityZones: ["eu-west-1b", "eu-west-1c"],
      singleNatGateway: false,
    });
    expect(names((await container(s, "api")).secrets)).toContain("SENTRY_DSN");
    expect(names((await container(s, "indexer")).secrets)).not.toContain("SENTRY_DSN");
    const policy = JSON.parse(await valueOf(s.executionRolePolicy.policy)) as { Statement: { Sid: string; Resource: string[] }[] };
    expect(policy.Statement.find((st) => st.Sid === "ReadReferencedSecrets")!.Resource).toContain(sentry);
    expect((await container(s, "migrations")).command).toEqual(["/app/migrate", "up"]);
    expect(await Promise.all(s.privateSubnets.map((sub) => valueOf(sub.availabilityZone)))).toEqual(["eu-west-1b", "eu-west-1c"]);
    expect(s.natGateways).toHaveLength(2);
    const natIds = await Promise.all(s.privateRoutes.map((r) => valueOf(r.natGatewayId)));
    expect(new Set(natIds).size).toBe(2);
  });

  it.each([
    ["a :latest image", { apiImage: "ghcr.io/example/sorolens-api:latest" }, /apiImage/],
    ["an untagged image", { indexerImage: "ghcr.io/example/sorolens-indexer" }, /indexerImage/],
    ["a Route 53 zone without a domain", { route53ZoneId: "Z0000000000000000000" }, /route53ZoneId/],
    ["three availability zones", { availabilityZones: ["a", "b", "c"] }, /availabilityZones/],
    ["zero API tasks", { apiDesiredCount: 0 }, /apiDesiredCount/],
    ["a small VPC", { vpcCidr: "10.0.0.0/24" }, /vpcCidr/],
    ["a non-16 database version", { dbEngineVersion: "15.4" }, /dbEngineVersion/],
    ["mainnet without an RPC endpoint", { stellarNetwork: "mainnet" as const }, /mainnet/],
  ])("rejects %s", (_label, args, message) => {
    expect(() => stack({ allowHttpOnly: true, ...args })).toThrow(SorolensConfigError);
    expect(() => stack({ allowHttpOnly: true, ...args })).toThrow(message);
  });
});
