import * as pulumi from "@pulumi/pulumi";

/**
 * Pulumi runtime mocks: every resource and provider call is answered
 * in-process, so tests need no cloud credentials, no backend and create
 * nothing. Computed attributes get realistic placeholder values.
 */
export async function installMocks(): Promise<void> {
  await pulumi.runtime.setMocks(
    {
      newResource(args: pulumi.runtime.MockResourceArgs) {
        const state: Record<string, unknown> = { ...args.inputs };
        switch (args.type) {
          case "aws:lb/loadBalancer:LoadBalancer":
            state.dnsName = "sorolens-alb-123456.us-east-1.elb.amazonaws.com";
            state.zoneId = "Z35SXDOTRQ7X7K";
            break;
          case "aws:rds/cluster:Cluster":
            state.endpoint = "sorolens-db.cluster-mock.us-east-1.rds.amazonaws.com";
            state.port = 5432;
            break;
          case "aws:elasticache/replicationGroup:ReplicationGroup":
            state.primaryEndpointAddress = "master.sorolens-redis.mock.use1.cache.amazonaws.com";
            break;
          case "random:index/randomPassword:RandomPassword":
            state.result = "MockGeneratedPassword0123456789ab";
            break;
        }
        state.arn ??= `arn:aws:mock:us-east-1:123456789012:${args.type.split(":")[1]}/${args.name}`;
        return { id: `${args.name}-id`, state };
      },
      call(args: pulumi.runtime.MockCallArgs) {
        switch (args.token) {
          case "aws:index/getAvailabilityZones:getAvailabilityZones":
            return { names: ["us-east-1a", "us-east-1b", "us-east-1c"] };
          case "aws:index/getRegion:getRegion":
            return { region: "us-east-1", name: "us-east-1" };
          default:
            return args.inputs;
        }
      },
    },
    "sorolens-test",
    "test",
    false,
  );
}

/** Resolves an Output inside a test. */
export function valueOf<T>(output: pulumi.Output<T>): Promise<T> {
  return new Promise((resolve) => output.apply(resolve));
}
