import * as pulumi from "@pulumi/pulumi";

import { SorolensAws, type StellarNetwork } from "../../src";

// Every setting comes from stack config (pulumi config set ...), mirroring
// deploy/terraform/examples/aws. The AWS region is the provider's aws:region.
const config = new pulumi.Config();

const sorolens = new SorolensAws("sorolens", {
  name: config.get("name") ?? "sorolens",

  apiImage: config.require("apiImage"),
  indexerImage: config.require("indexerImage"),
  dashboardImage: config.require("dashboardImage"),
  migrationsCommand: config.getObject<string[]>("migrationsCommand"),

  certificateArn: config.get("certificateArn"),
  allowHttpOnly: config.getBoolean("allowHttpOnly") ?? false,
  domainName: config.get("domainName"),
  route53ZoneId: config.get("route53ZoneId"),
  availabilityZones: config.getObject<string[]>("availabilityZones"),

  stellarNetwork: (config.get("stellarNetwork") ?? "testnet") as StellarNetwork,
  sorobanRpcUrls: config.getObject<Partial<Record<StellarNetwork, string>>>("sorobanRpcUrls"),
  watchdogContractId: config.get("watchdogContractId"),

  dbDeletionProtection: config.getBoolean("dbDeletionProtection") ?? true,
  dbSkipFinalSnapshot: config.getBoolean("dbSkipFinalSnapshot") ?? false,
});

export const url = sorolens.url;
export const healthUrl = sorolens.healthUrl;
export const loadBalancerDnsName = sorolens.loadBalancerDnsName;
export const dashboardBuildApiUrl = sorolens.dashboardBuildApiUrl;
export const clusterName = sorolens.clusterName;
export const migrationsRunTaskCommand = sorolens.migrationsRunTaskCommand;
