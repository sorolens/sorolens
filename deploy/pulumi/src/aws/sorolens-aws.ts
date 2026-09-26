import * as aws from "@pulumi/aws";
import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

import { API_PORT, AppConfigArgs, DASHBOARD_PORT, Env, SorolensConfigError, buildAppConfig } from "../core/config";
import { cidrSubnet, isIpv4Cidr } from "./cidr";

/**
 * Inputs of SorolensAws. Mirrors the variables of
 * deploy/terraform/modules/aws (camelCase instead of snake_case).
 */
export interface SorolensAwsArgs extends AppConfigArgs {
  /** Tags added to every resource that supports them. */
  tags?: Record<string, string>;

  /** API image with an explicit tag (not :latest) or @sha256 digest. Images are not published yet (#96, #381). */
  apiImage: string;
  /** Indexer image; its entrypoint must be the indexer binary. */
  indexerImage: string;
  /** Dashboard image, built with NEXT_PUBLIC_API_URL set to this deployment's URL. */
  dashboardImage: string;
  /** Image for the one-off migrations task. Defaults to apiImage. */
  migrationsImage?: string;
  /** Command that applies migrations and exits (depends on #381). Unset keeps the image's default command. */
  migrationsCommand?: string[];
  /** Secrets Manager ARN with {"username","password"} for a private registry. */
  imagePullSecretArn?: string;
  /** "X86_64" (default) or "ARM64". */
  cpuArchitecture?: "X86_64" | "ARM64";

  /** Extra plain environment for API and migrations (no secrets). */
  apiExtraEnvironment?: Record<string, string>;
  /** Extra secrets for API and migrations: env var name => Secrets Manager ARN. */
  apiExtraSecrets?: Record<string, string>;
  /** Extra plain environment for the indexer (no secrets). */
  indexerExtraEnvironment?: Record<string, string>;
  /** Extra secrets for the indexer: env var name => Secrets Manager ARN. */
  indexerExtraSecrets?: Record<string, string>;

  apiCpu?: number;
  apiMemory?: number;
  /** 1-20, default 1. */
  apiDesiredCount?: number;
  dashboardCpu?: number;
  dashboardMemory?: number;
  /** 1-20, default 1. */
  dashboardDesiredCount?: number;
  /** The indexer always runs as exactly one task. */
  indexerCpu?: number;
  indexerMemory?: number;

  /** VPC CIDR, /20 or larger. Default 10.40.0.0/16. */
  vpcCidr?: string;
  /** Exactly two zones, or empty for the region's first two. */
  availabilityZones?: string[];
  /** One NAT gateway for both zones (default) instead of one per zone. */
  singleNatGateway?: boolean;

  /** ACM certificate ARN for HTTPS. Required unless allowHttpOnly is true. */
  certificateArn?: string;
  /** Explicit opt-in to plain HTTP without a certificate (evaluation only). */
  allowHttpOnly?: boolean;
  /** Public hostname, e.g. sorolens.example.com. */
  domainName?: string;
  /** Route 53 zone in which to create the domainName alias record. */
  route53ZoneId?: string;
  /** IPv4 CIDRs allowed to reach the load balancer. Default 0.0.0.0/0. */
  albIngressCidrs?: string[];
  albDeletionProtection?: boolean;

  /** Aurora PostgreSQL 16.x version. Default "16.6". */
  dbEngineVersion?: string;
  /** Serverless v2 ACUs. Defaults 0.5 and 4. */
  dbMinCapacity?: number;
  dbMaxCapacity?: number;
  /** 1-3 instances, default 1. */
  dbInstanceCount?: number;
  /** 1-35 days, default 7. */
  dbBackupRetentionDays?: number;
  /** Default true. */
  dbDeletionProtection?: boolean;
  /** Default false. */
  dbSkipFinalSnapshot?: boolean;

  /** Default cache.t4g.micro. */
  redisNodeType?: string;
  /** Default "7.1". */
  redisEngineVersion?: string;
  /** 1-3 nodes, default 1; 2+ enables failover. */
  redisNumCacheClusters?: number;

  /** CloudWatch retention, default 30 days. */
  logRetentionDays?: number;
  enableContainerInsights?: boolean;
  /** 0 or 7-30, default 7. */
  secretRecoveryWindowDays?: number;
}

type ServiceKey = "api" | "dashboard" | "indexer" | "migrations";
const SERVICE_KEYS: ServiceKey[] = ["api", "dashboard", "indexer", "migrations"];

const IMAGE_RE = /^\S+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$/;

function requireImage(field: string, value: string | undefined): string {
  if (!value || !IMAGE_RE.test(value) || value.endsWith(":latest")) {
    throw new SorolensConfigError(`${field} must include an explicit tag (not :latest) or an @sha256 digest`);
  }
  return value;
}

function requireRange(field: string, value: number, min: number, max: number, integer = true): number {
  if (!(value >= min && value <= max) || (integer && !Number.isInteger(value))) {
    throw new SorolensConfigError(`${field} must be ${integer ? "a whole number " : ""}between ${min} and ${max} (got ${value})`);
  }
  return value;
}

/**
 * A complete self-hosted Sorolens stack on AWS: VPC, Aurora PostgreSQL
 * Serverless v2, ElastiCache Redis, Secrets Manager, ECS Fargate services
 * (API, dashboard, one indexer), a migrations task and an Application Load
 * Balancer. Secure by default: private data stores, encryption at rest and
 * in transit, HTTPS unless explicitly opted out, least-privilege IAM.
 */
export class SorolensAws extends pulumi.ComponentResource {
  /** Public URL of the dashboard; the API is under /api/v1 on the same origin. */
  readonly url: pulumi.Output<string>;
  readonly healthUrl: pulumi.Output<string>;
  readonly loadBalancerDnsName: pulumi.Output<string>;
  /** NEXT_PUBLIC_API_URL to build the dashboard image with. */
  readonly dashboardBuildApiUrl: pulumi.Output<string>;
  readonly clusterName: pulumi.Output<string>;
  readonly migrationsRunTaskCommand: pulumi.Output<string>;
  readonly databaseUrlSecretArn: pulumi.Output<string>;
  readonly redisUrlSecretArn: pulumi.Output<string>;

  // Exposed for inspection (and tests).
  readonly vpc: aws.ec2.Vpc;
  readonly privateSubnets: aws.ec2.Subnet[];
  readonly natGateways: aws.ec2.NatGateway[];
  readonly privateRoutes: aws.ec2.Route[];
  readonly database: aws.rds.Cluster;
  readonly databaseInstances: aws.rds.ClusterInstance[];
  readonly redis: aws.elasticache.ReplicationGroup;
  readonly databaseUrlSecretVersion: aws.secretsmanager.SecretVersion;
  readonly redisUrlSecretVersion: aws.secretsmanager.SecretVersion;
  readonly dbIngressRules: aws.vpc.SecurityGroupIngressRule[];
  readonly redisIngressRules: aws.vpc.SecurityGroupIngressRule[];
  readonly executionRolePolicy: aws.iam.RolePolicy;
  readonly taskRole: aws.iam.Role;
  readonly taskDefinitions: Record<ServiceKey, aws.ecs.TaskDefinition>;
  readonly services: { api: aws.ecs.Service; dashboard: aws.ecs.Service; indexer: aws.ecs.Service };
  readonly loadBalancer: aws.lb.LoadBalancer;
  readonly httpsListener?: aws.lb.Listener;
  readonly httpRedirectListener?: aws.lb.Listener;
  readonly httpListener?: aws.lb.Listener;
  readonly dnsRecord?: aws.route53.Record;

  constructor(name: string, args: SorolensAwsArgs, opts?: pulumi.ComponentResourceOptions) {
    super("sorolens:aws:Stack", name, {}, opts);

    // ---- validation (mirrors the Terraform variable validations) ----------
    // Everything is checked before the first resource is registered, so bad
    // input never leaves a half-declared stack. The app config is built again
    // below once the public URL (load balancer DNS name) exists.
    buildAppConfig(args, "");
    const apiImage = requireImage("apiImage", args.apiImage);
    const indexerImage = requireImage("indexerImage", args.indexerImage);
    const dashboardImage = requireImage("dashboardImage", args.dashboardImage);
    const migrationsImage = args.migrationsImage ? requireImage("migrationsImage", args.migrationsImage) : apiImage;

    if (!args.certificateArn && !args.allowHttpOnly) {
      throw new SorolensConfigError(
        "Set certificateArn to serve HTTPS, or set allowHttpOnly: true to explicitly accept plain HTTP for an evaluation stack.",
      );
    }
    if (args.route53ZoneId && !args.domainName) {
      throw new SorolensConfigError("route53ZoneId is set but domainName is not: the alias record needs a name.");
    }

    const vpcCidr = args.vpcCidr ?? "10.40.0.0/16";
    if (!isIpv4Cidr(vpcCidr) || Number(vpcCidr.split("/")[1]) > 20) {
      throw new SorolensConfigError("vpcCidr must be a valid IPv4 CIDR of /20 or larger");
    }
    const zonesGiven = args.availabilityZones ?? [];
    if (zonesGiven.length !== 0 && zonesGiven.length !== 2) {
      throw new SorolensConfigError("availabilityZones must be empty or list exactly two zones");
    }
    const albIngressCidrs = args.albIngressCidrs ?? ["0.0.0.0/0"];
    if (!albIngressCidrs.every(isIpv4Cidr)) {
      throw new SorolensConfigError("albIngressCidrs must contain IPv4 CIDR blocks");
    }

    const apiDesiredCount = requireRange("apiDesiredCount", args.apiDesiredCount ?? 1, 1, 20);
    const dashboardDesiredCount = requireRange("dashboardDesiredCount", args.dashboardDesiredCount ?? 1, 1, 20);
    const dbInstanceCount = requireRange("dbInstanceCount", args.dbInstanceCount ?? 1, 1, 3);
    const dbMaxCapacity = requireRange("dbMaxCapacity", args.dbMaxCapacity ?? 4, 1, 256, false);
    const dbMinCapacity = requireRange("dbMinCapacity", args.dbMinCapacity ?? 0.5, 0, dbMaxCapacity, false);
    const dbBackupRetentionDays = requireRange("dbBackupRetentionDays", args.dbBackupRetentionDays ?? 7, 1, 35);
    const redisNumCacheClusters = requireRange("redisNumCacheClusters", args.redisNumCacheClusters ?? 1, 1, 3);
    const secretRecoveryWindowDays = args.secretRecoveryWindowDays ?? 7;
    if (secretRecoveryWindowDays !== 0) requireRange("secretRecoveryWindowDays", secretRecoveryWindowDays, 7, 30);
    const dbEngineVersion = args.dbEngineVersion ?? "16.6";
    if (!/^16(\.[0-9]+)?$/.test(dbEngineVersion)) {
      throw new SorolensConfigError("dbEngineVersion must be an Aurora PostgreSQL 16.x version");
    }
    const cpuArchitecture = args.cpuArchitecture ?? "X86_64";
    if (cpuArchitecture !== "X86_64" && cpuArchitecture !== "ARM64") {
      throw new SorolensConfigError("cpuArchitecture must be X86_64 or ARM64");
    }

    const prefix = args.name ?? "sorolens";
    const parent = { parent: this };
    const tags = { "sorolens:stack": prefix, ...(args.tags ?? {}) };
    const nameTag = (n: string) => ({ ...tags, Name: n });

    // ---- network --------------------------------------------------------------
    const azs: pulumi.Output<string[]> =
      zonesGiven.length === 2
        ? pulumi.output(zonesGiven)
        : aws.getAvailabilityZonesOutput({ state: "available" }, parent).names.apply((n) => n.slice(0, 2));
    const az = (i: number) => azs.apply((z) => z[i] as string);

    this.vpc = new aws.ec2.Vpc(`${name}-vpc`, {
      cidrBlock: vpcCidr,
      enableDnsSupport: true,
      enableDnsHostnames: true,
      tags: nameTag(prefix),
    }, parent);

    // No ingress/egress arguments: the provider removes every rule from the
    // VPC's default security group.
    new aws.ec2.DefaultSecurityGroup(`${name}-default-sg`, {
      vpcId: this.vpc.id,
      tags: nameTag(`${prefix}-default-deny`),
    }, parent);

    const igw = new aws.ec2.InternetGateway(`${name}-igw`, { vpcId: this.vpc.id, tags: nameTag(prefix) }, parent);

    const publicSubnets = [0, 1].map((i) => new aws.ec2.Subnet(`${name}-public-${i}`, {
      vpcId: this.vpc.id,
      availabilityZone: az(i),
      cidrBlock: cidrSubnet(vpcCidr, 4, i),
      tags: az(i).apply((z) => nameTag(`${prefix}-public-${z}`)),
    }, parent));

    this.privateSubnets = [0, 1].map((i) => new aws.ec2.Subnet(`${name}-private-${i}`, {
      vpcId: this.vpc.id,
      availabilityZone: az(i),
      cidrBlock: cidrSubnet(vpcCidr, 4, i + 8),
      tags: az(i).apply((z) => nameTag(`${prefix}-private-${z}`)),
    }, parent));
    const privateSubnetIds = this.privateSubnets.map((s) => s.id);

    const publicRouteTable = new aws.ec2.RouteTable(`${name}-public-rt`, { vpcId: this.vpc.id, tags: nameTag(`${prefix}-public`) }, parent);
    new aws.ec2.Route(`${name}-public-internet`, {
      routeTableId: publicRouteTable.id,
      destinationCidrBlock: "0.0.0.0/0",
      gatewayId: igw.id,
    }, parent);
    publicSubnets.forEach((s, i) => new aws.ec2.RouteTableAssociation(`${name}-public-${i}`, {
      subnetId: s.id,
      routeTableId: publicRouteTable.id,
    }, parent));

    const natCount = args.singleNatGateway === false ? 2 : 1;
    this.natGateways = Array.from({ length: natCount }, (_, i) => {
      const eip = new aws.ec2.Eip(`${name}-nat-${i}`, { domain: "vpc", tags: nameTag(`${prefix}-nat-${i}`) }, parent);
      return new aws.ec2.NatGateway(`${name}-nat-${i}`, {
        allocationId: eip.id,
        subnetId: publicSubnets[i]!.id,
        tags: nameTag(`${prefix}-${i}`),
      }, { parent: this, dependsOn: [igw] });
    });

    // Private subnets reach the internet (RPC, registry, AWS APIs) only via NAT.
    this.privateRoutes = this.privateSubnets.map((s, i) => {
      const rt = new aws.ec2.RouteTable(`${name}-private-rt-${i}`, {
        vpcId: this.vpc.id,
        tags: az(i).apply((z) => nameTag(`${prefix}-private-${z}`)),
      }, parent);
      new aws.ec2.RouteTableAssociation(`${name}-private-${i}`, { subnetId: s.id, routeTableId: rt.id }, parent);
      return new aws.ec2.Route(`${name}-private-nat-${i}`, {
        routeTableId: rt.id,
        destinationCidrBlock: "0.0.0.0/0",
        natGatewayId: this.natGateways[natCount === 1 ? 0 : i]!.id,
      }, parent);
    });

    // ---- security groups ------------------------------------------------------
    const sg = (key: string, description: string) => new aws.ec2.SecurityGroup(`${name}-${key}-sg`, {
      name: `${prefix}-${key}`,
      description,
      vpcId: this.vpc.id,
      tags: nameTag(`${prefix}-${key}`),
    }, parent);
    const albSg = sg("alb", "Sorolens load balancer: HTTP/HTTPS from albIngressCidrs");
    const webSg = sg("web", "Sorolens API and dashboard tasks: traffic from the load balancer only");
    const workerSg = sg("worker", "Sorolens indexer and migrations tasks: no inbound traffic");
    const dbSg = sg("db", "Sorolens Aurora PostgreSQL: 5432 from Sorolens tasks only");
    const redisSg = sg("redis", "Sorolens ElastiCache Redis: 6379 from Sorolens tasks only");

    const listenerPorts = args.certificateArn ? [80, 443] : [80];
    for (const cidr of albIngressCidrs) {
      for (const port of listenerPorts) {
        new aws.vpc.SecurityGroupIngressRule(`${name}-alb-in-${cidr}-${port}`, {
          securityGroupId: albSg.id,
          description: `HTTP(S) from ${cidr}`,
          cidrIpv4: cidr,
          ipProtocol: "tcp",
          fromPort: port,
          toPort: port,
        }, parent);
      }
    }

    for (const port of [API_PORT, DASHBOARD_PORT]) {
      new aws.vpc.SecurityGroupEgressRule(`${name}-alb-to-web-${port}`, {
        securityGroupId: albSg.id,
        description: `To Sorolens tasks on ${port}`,
        referencedSecurityGroupId: webSg.id,
        ipProtocol: "tcp",
        fromPort: port,
        toPort: port,
      }, parent);
      new aws.vpc.SecurityGroupIngressRule(`${name}-web-from-alb-${port}`, {
        securityGroupId: webSg.id,
        description: `From the load balancer on ${port}`,
        referencedSecurityGroupId: albSg.id,
        ipProtocol: "tcp",
        fromPort: port,
        toPort: port,
      }, parent);
    }

    const dataClients = { web: webSg, worker: workerSg };
    for (const [key, group] of Object.entries(dataClients)) {
      new aws.vpc.SecurityGroupEgressRule(`${name}-${key}-outbound`, {
        securityGroupId: group.id,
        description: "Outbound for RPC, registry and AWS APIs",
        cidrIpv4: "0.0.0.0/0",
        ipProtocol: "-1",
      }, parent);
    }
    this.dbIngressRules = Object.entries(dataClients).map(([key, group]) => new aws.vpc.SecurityGroupIngressRule(`${name}-db-from-${key}`, {
      securityGroupId: dbSg.id,
      description: `PostgreSQL from ${key} tasks`,
      referencedSecurityGroupId: group.id,
      ipProtocol: "tcp",
      fromPort: 5432,
      toPort: 5432,
    }, parent));
    this.redisIngressRules = Object.entries(dataClients).map(([key, group]) => new aws.vpc.SecurityGroupIngressRule(`${name}-redis-from-${key}`, {
      securityGroupId: redisSg.id,
      description: `Redis from ${key} tasks`,
      referencedSecurityGroupId: group.id,
      ipProtocol: "tcp",
      fromPort: 6379,
      toPort: 6379,
    }, parent));

    // ---- database ---------------------------------------------------------------
    // Alphanumeric so the password is safe inside a connection URL.
    const dbPassword = new random.RandomPassword(`${name}-db-password`, { length: 32, special: false }, parent);
    const dbSubnetGroup = new aws.rds.SubnetGroup(`${name}-db`, { name: `${prefix}-db`, subnetIds: privateSubnetIds, tags }, parent);
    const dbSkipFinalSnapshot = args.dbSkipFinalSnapshot ?? false;

    this.database = new aws.rds.Cluster(`${name}-db`, {
      clusterIdentifier: `${prefix}-db`,
      engine: "aurora-postgresql",
      engineMode: "provisioned",
      engineVersion: dbEngineVersion,
      databaseName: "sorolens",
      masterUsername: "sorolens",
      masterPassword: dbPassword.result,
      dbSubnetGroupName: dbSubnetGroup.name,
      vpcSecurityGroupIds: [dbSg.id],
      storageEncrypted: true,
      iamDatabaseAuthenticationEnabled: false,
      enabledCloudwatchLogsExports: ["postgresql"],
      backupRetentionPeriod: dbBackupRetentionDays,
      copyTagsToSnapshot: true,
      deletionProtection: args.dbDeletionProtection ?? true,
      skipFinalSnapshot: dbSkipFinalSnapshot,
      finalSnapshotIdentifier: dbSkipFinalSnapshot ? undefined : `${prefix}-db-final`,
      serverlessv2ScalingConfiguration: { minCapacity: dbMinCapacity, maxCapacity: dbMaxCapacity },
      tags,
    }, parent);

    this.databaseInstances = Array.from({ length: dbInstanceCount }, (_, i) => new aws.rds.ClusterInstance(`${name}-db-${i}`, {
      identifier: `${prefix}-db-${i}`,
      clusterIdentifier: this.database.id,
      instanceClass: "db.serverless",
      engine: "aurora-postgresql",
      engineVersion: this.database.engineVersion,
      dbSubnetGroupName: dbSubnetGroup.name,
      publiclyAccessible: false,
      autoMinorVersionUpgrade: true,
      tags,
    }, parent));

    // ---- redis ----------------------------------------------------------------
    const redisToken = new random.RandomPassword(`${name}-redis-token`, { length: 32, special: false }, parent);
    const redisSubnetGroup = new aws.elasticache.SubnetGroup(`${name}-redis`, { name: `${prefix}-redis`, subnetIds: privateSubnetIds, tags }, parent);
    // Own parameter group (the default one cannot be edited).
    const redisParams = new aws.elasticache.ParameterGroup(`${name}-redis7`, { name: `${prefix}-redis7`, family: "redis7", tags }, parent);

    this.redis = new aws.elasticache.ReplicationGroup(`${name}-redis`, {
      replicationGroupId: `${prefix}-redis`,
      description: `Sorolens ${prefix} Redis`,
      engine: "redis",
      engineVersion: args.redisEngineVersion ?? "7.1",
      nodeType: args.redisNodeType ?? "cache.t4g.micro",
      numCacheClusters: redisNumCacheClusters,
      parameterGroupName: redisParams.name,
      port: 6379,
      subnetGroupName: redisSubnetGroup.name,
      securityGroupIds: [redisSg.id],
      atRestEncryptionEnabled: true,
      transitEncryptionEnabled: true,
      authToken: redisToken.result,
      automaticFailoverEnabled: redisNumCacheClusters > 1,
      multiAzEnabled: redisNumCacheClusters > 1,
      tags,
    }, parent);

    // ---- secrets ----------------------------------------------------------------
    // Connection URLs reach the containers only as ECS secrets. The passwords
    // are also in Pulumi state, where Pulumi encrypts secret values.
    const secret = (key: string, description: string) => new aws.secretsmanager.Secret(`${name}-${key}`, {
      name: `${prefix}/${key}`,
      description,
      recoveryWindowInDays: secretRecoveryWindowDays,
      tags,
    }, parent);
    const databaseUrlSecret = secret("database-url", `Sorolens ${prefix} PostgreSQL connection URL (DATABASE_URL / DIRECT_DATABASE_URL)`);
    const redisUrlSecret = secret("redis-url", `Sorolens ${prefix} Redis connection URL (REDIS_URL, TLS)`);

    this.databaseUrlSecretVersion = new aws.secretsmanager.SecretVersion(`${name}-database-url`, {
      secretId: databaseUrlSecret.id,
      secretString: pulumi.secret(pulumi.interpolate`postgres://${this.database.masterUsername}:${dbPassword.result}@${this.database.endpoint}:${this.database.port}/${this.database.databaseName}?sslmode=require`),
    }, parent);
    // rediss:// = Redis over TLS, which go-redis understands natively.
    this.redisUrlSecretVersion = new aws.secretsmanager.SecretVersion(`${name}-redis-url`, {
      secretId: redisUrlSecret.id,
      secretString: pulumi.secret(pulumi.interpolate`rediss://:${redisToken.result}@${this.redis.primaryEndpointAddress}:${this.redis.port}`),
    }, parent);

    // ---- logs & IAM -------------------------------------------------------------
    const logGroups = Object.fromEntries(SERVICE_KEYS.map((k) => [k, new aws.cloudwatch.LogGroup(`${name}-${k}`, {
      name: `/ecs/${prefix}/${k}`,
      retentionInDays: args.logRetentionDays ?? 30,
      tags,
    }, parent)])) as Record<ServiceKey, aws.cloudwatch.LogGroup>;

    const assumeRolePolicy = JSON.stringify({
      Version: "2012-10-17",
      Statement: [{ Effect: "Allow", Principal: { Service: "ecs-tasks.amazonaws.com" }, Action: "sts:AssumeRole" }],
    });

    const executionRole = new aws.iam.Role(`${name}-ecs-execution`, { name: `${prefix}-ecs-execution`, assumeRolePolicy, tags }, parent);
    // Sorolens calls no AWS APIs, so the containers' role has no permissions.
    this.taskRole = new aws.iam.Role(`${name}-ecs-task`, { name: `${prefix}-ecs-task`, assumeRolePolicy, tags }, parent);

    const extraSecretArns = [...Object.values(args.apiExtraSecrets ?? {}), ...Object.values(args.indexerExtraSecrets ?? {})];
    const readableSecretArns = pulumi
      .all([databaseUrlSecret.arn, redisUrlSecret.arn])
      .apply((own) => [...new Set([...own, ...(args.imagePullSecretArn ? [args.imagePullSecretArn] : []), ...extraSecretArns])]);

    this.executionRolePolicy = new aws.iam.RolePolicy(`${name}-ecs-execution`, {
      name: "pull-images-write-logs-read-secrets",
      role: executionRole.id,
      policy: pulumi.jsonStringify({
        Version: "2012-10-17",
        Statement: [
          {
            Sid: "PullFromEcr",
            Effect: "Allow",
            Action: ["ecr:GetAuthorizationToken", "ecr:BatchCheckLayerAvailability", "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage"],
            Resource: "*",
          },
          {
            Sid: "WriteContainerLogs",
            Effect: "Allow",
            Action: ["logs:CreateLogStream", "logs:PutLogEvents"],
            Resource: SERVICE_KEYS.map((k) => pulumi.interpolate`${logGroups[k].arn}:*`),
          },
          {
            Sid: "ReadReferencedSecrets",
            Effect: "Allow",
            Action: ["secretsmanager:GetSecretValue"],
            Resource: readableSecretArns,
          },
        ],
      }),
    }, parent);

    // ---- load balancer ------------------------------------------------------------
    this.loadBalancer = new aws.lb.LoadBalancer(`${name}-alb`, {
      name: `${prefix}-alb`,
      loadBalancerType: "application",
      internal: false,
      subnets: publicSubnets.map((s) => s.id),
      securityGroups: [albSg.id],
      dropInvalidHeaderFields: true,
      enableDeletionProtection: args.albDeletionProtection ?? false,
      tags,
    }, parent);

    const scheme = args.certificateArn ? "https" : "http";
    const publicHost: pulumi.Output<string> = args.domainName ? pulumi.output(args.domainName) : this.loadBalancer.dnsName;
    const publicUrl = pulumi.interpolate`${scheme}://${publicHost}`;
    const app = buildAppConfig(args, publicUrl);

    const targetGroup = (key: string, port: number, path: string, matcher: string) => new aws.lb.TargetGroup(`${name}-${key}`, {
      name: `${prefix}-${key}`,
      port,
      protocol: "HTTP",
      targetType: "ip",
      vpcId: this.vpc.id,
      deregistrationDelay: 30,
      healthCheck: { path, matcher, interval: 15, healthyThreshold: 2, unhealthyThreshold: 3 },
      tags,
    }, parent);
    const apiTg = targetGroup("api", app.apiPort, "/health", "200");
    const dashboardTg = targetGroup("dashboard", app.dashboardPort, "/", "200-399");

    let mainListener: aws.lb.Listener;
    if (args.certificateArn) {
      this.httpsListener = new aws.lb.Listener(`${name}-https`, {
        loadBalancerArn: this.loadBalancer.arn,
        port: 443,
        protocol: "HTTPS",
        sslPolicy: "ELBSecurityPolicy-TLS13-1-2-2021-06",
        certificateArn: args.certificateArn,
        defaultActions: [{ type: "forward", targetGroupArn: dashboardTg.arn }],
        tags,
      }, parent);
      this.httpRedirectListener = new aws.lb.Listener(`${name}-http-redirect`, {
        loadBalancerArn: this.loadBalancer.arn,
        port: 80,
        protocol: "HTTP",
        defaultActions: [{ type: "redirect", redirect: { port: "443", protocol: "HTTPS", statusCode: "HTTP_301" } }],
        tags,
      }, parent);
      mainListener = this.httpsListener;
    } else {
      this.httpListener = new aws.lb.Listener(`${name}-http`, {
        loadBalancerArn: this.loadBalancer.arn,
        port: 80,
        protocol: "HTTP",
        defaultActions: [{ type: "forward", targetGroupArn: dashboardTg.arn }],
        tags,
      }, parent);
      mainListener = this.httpListener;
    }

    const apiRule = new aws.lb.ListenerRule(`${name}-api`, {
      listenerArn: mainListener.arn,
      priority: 10,
      actions: [{ type: "forward", targetGroupArn: apiTg.arn }],
      conditions: [{ pathPattern: { values: ["/api/*", "/health", "/readyz"] } }],
      tags,
    }, parent);

    if (args.route53ZoneId && args.domainName) {
      this.dnsRecord = new aws.route53.Record(`${name}-dns`, {
        zoneId: args.route53ZoneId,
        name: args.domainName,
        type: "A",
        aliases: [{ name: this.loadBalancer.dnsName, zoneId: this.loadBalancer.zoneId, evaluateTargetHealth: true }],
      }, parent);
    }

    // ---- ECS ------------------------------------------------------------------------
    const cluster = new aws.ecs.Cluster(`${name}-cluster`, {
      name: prefix,
      settings: [{ name: "containerInsights", value: args.enableContainerInsights ? "enabled" : "disabled" }],
      tags,
    }, parent);

    const connectionSecrets: Env = { DATABASE_URL: databaseUrlSecret.arn, REDIS_URL: redisUrlSecret.arn };
    // DIRECT_DATABASE_URL bypasses poolers for migrations; on Aurora it is the same URL.
    const apiSecrets: Env = { ...connectionSecrets, DIRECT_DATABASE_URL: databaseUrlSecret.arn, ...(args.apiExtraSecrets ?? {}) };
    const indexerSecrets: Env = { ...connectionSecrets, ...(args.indexerExtraSecrets ?? {}) };
    const apiEnvironment: Env = { ...app.apiEnvironment, ...(args.apiExtraEnvironment ?? {}) };
    const indexerEnvironment: Env = { ...app.indexerEnvironment, ...(args.indexerExtraEnvironment ?? {}) };

    const containers: Record<ServiceKey, { image: string; command?: string[]; environment: Env; secrets: Env; port?: number }> = {
      api: { image: apiImage, environment: apiEnvironment, secrets: apiSecrets, port: app.apiPort },
      dashboard: { image: dashboardImage, environment: app.dashboardEnvironment, secrets: {}, port: app.dashboardPort },
      indexer: { image: indexerImage, command: app.indexerCommand, environment: indexerEnvironment, secrets: indexerSecrets },
      migrations: { image: migrationsImage, command: args.migrationsCommand, environment: apiEnvironment, secrets: apiSecrets },
    };
    const sizes: Record<ServiceKey, { cpu: number; memory: number }> = {
      api: { cpu: args.apiCpu ?? 256, memory: args.apiMemory ?? 512 },
      dashboard: { cpu: args.dashboardCpu ?? 256, memory: args.dashboardMemory ?? 512 },
      indexer: { cpu: args.indexerCpu ?? 256, memory: args.indexerMemory ?? 512 },
      migrations: { cpu: args.apiCpu ?? 256, memory: args.apiMemory ?? 512 },
    };
    const region = aws.getRegionOutput({}, parent).region;
    const sortedPairs = (env: Env) => Object.keys(env).sort().map((k) => ({ name: k, value: env[k] }));

    this.taskDefinitions = Object.fromEntries(SERVICE_KEYS.map((k) => {
      const c = containers[k];
      const definition = {
        name: k,
        image: c.image,
        essential: true,
        portMappings: c.port ? [{ containerPort: c.port, protocol: "tcp" }] : [],
        environment: sortedPairs(c.environment),
        secrets: Object.keys(c.secrets).sort().map((n) => ({ name: n, valueFrom: c.secrets[n] })),
        logConfiguration: {
          logDriver: "awslogs",
          options: { "awslogs-group": logGroups[k].name, "awslogs-region": region, "awslogs-stream-prefix": k },
        },
        ...(c.command ? { command: c.command } : {}),
        ...(args.imagePullSecretArn ? { repositoryCredentials: { credentialsParameter: args.imagePullSecretArn } } : {}),
      };
      return [k, new aws.ecs.TaskDefinition(`${name}-${k}`, {
        family: `${prefix}-${k}`,
        requiresCompatibilities: ["FARGATE"],
        networkMode: "awsvpc",
        cpu: String(sizes[k].cpu),
        memory: String(sizes[k].memory),
        executionRoleArn: executionRole.arn,
        taskRoleArn: this.taskRole.arn,
        runtimePlatform: { operatingSystemFamily: "LINUX", cpuArchitecture },
        containerDefinitions: pulumi.jsonStringify([definition]),
        tags,
      }, parent)];
    })) as Record<ServiceKey, aws.ecs.TaskDefinition>;

    const privateNetwork = (group: aws.ec2.SecurityGroup) => ({
      subnets: privateSubnetIds,
      securityGroups: [group.id],
      assignPublicIp: false,
    });
    const circuitBreaker = { enable: true, rollback: true };

    this.services = {
      api: new aws.ecs.Service(`${name}-api`, {
        name: `${prefix}-api`,
        cluster: cluster.arn,
        taskDefinition: this.taskDefinitions.api.arn,
        desiredCount: apiDesiredCount,
        launchType: "FARGATE",
        healthCheckGracePeriodSeconds: 60,
        enableExecuteCommand: false,
        networkConfiguration: privateNetwork(webSg),
        loadBalancers: [{ targetGroupArn: apiTg.arn, containerName: "api", containerPort: app.apiPort }],
        deploymentCircuitBreaker: circuitBreaker,
        tags,
      }, { parent: this, dependsOn: [apiRule] }),
      dashboard: new aws.ecs.Service(`${name}-dashboard`, {
        name: `${prefix}-dashboard`,
        cluster: cluster.arn,
        taskDefinition: this.taskDefinitions.dashboard.arn,
        desiredCount: dashboardDesiredCount,
        launchType: "FARGATE",
        healthCheckGracePeriodSeconds: 60,
        enableExecuteCommand: false,
        networkConfiguration: privateNetwork(webSg),
        loadBalancers: [{ targetGroupArn: dashboardTg.arn, containerName: "dashboard", containerPort: app.dashboardPort }],
        deploymentCircuitBreaker: circuitBreaker,
        tags,
      }, { parent: this, dependsOn: [mainListener] }),
      // Exactly one indexer; deployments stop the old task before starting the
      // new one, so two indexers never overlap.
      indexer: new aws.ecs.Service(`${name}-indexer`, {
        name: `${prefix}-indexer`,
        cluster: cluster.arn,
        taskDefinition: this.taskDefinitions.indexer.arn,
        desiredCount: 1,
        launchType: "FARGATE",
        deploymentMinimumHealthyPercent: 0,
        deploymentMaximumPercent: 100,
        enableExecuteCommand: false,
        networkConfiguration: privateNetwork(workerSg),
        deploymentCircuitBreaker: circuitBreaker,
        tags,
      }, parent),
    };

    // ---- outputs ------------------------------------------------------------------
    this.url = publicUrl;
    this.healthUrl = pulumi.interpolate`${publicUrl}/health`;
    this.loadBalancerDnsName = this.loadBalancer.dnsName;
    this.dashboardBuildApiUrl = publicUrl;
    this.clusterName = cluster.name;
    this.databaseUrlSecretArn = databaseUrlSecret.arn;
    this.redisUrlSecretArn = redisUrlSecret.arn;
    this.migrationsRunTaskCommand = pulumi.interpolate`aws ecs run-task --cluster ${cluster.name} --task-definition ${this.taskDefinitions.migrations.arn} --launch-type FARGATE --network-configuration 'awsvpcConfiguration={subnets=[${pulumi.all(privateSubnetIds).apply((ids) => ids.join(","))}],securityGroups=[${workerSg.id}],assignPublicIp=DISABLED}'`;

    this.registerOutputs({
      url: this.url,
      healthUrl: this.healthUrl,
      loadBalancerDnsName: this.loadBalancerDnsName,
      dashboardBuildApiUrl: this.dashboardBuildApiUrl,
      clusterName: this.clusterName,
      migrationsRunTaskCommand: this.migrationsRunTaskCommand,
      databaseUrlSecretArn: this.databaseUrlSecretArn,
      redisUrlSecretArn: this.redisUrlSecretArn,
    });
  }
}
