import { describe, expect, it } from "vitest";

import { cidrSubnet, isIpv4Cidr } from "../src/aws/cidr";

describe("cidrSubnet", () => {
  // Expected values are what Terraform's cidrsubnet() returns, so the Pulumi
  // and Terraform stacks lay out the same subnets.
  it.each([
    ["10.40.0.0/16", 4, 0, "10.40.0.0/20"],
    ["10.40.0.0/16", 4, 1, "10.40.16.0/20"],
    ["10.40.0.0/16", 4, 8, "10.40.128.0/20"],
    ["10.40.0.0/16", 4, 9, "10.40.144.0/20"],
    ["172.16.0.0/12", 4, 15, "172.31.0.0/16"],
    ["10.0.0.0/20", 4, 9, "10.0.9.0/24"],
  ])("cidrSubnet(%s, %i, %i) = %s", (prefix, bits, num, expected) => {
    expect(cidrSubnet(prefix, bits, num)).toBe(expected);
  });

  it("rejects subnet numbers that do not fit", () => {
    expect(() => cidrSubnet("10.40.0.0/16", 4, 16)).toThrow();
    expect(() => cidrSubnet("10.40.0.0/30", 4, 0)).toThrow();
  });

  it("validates IPv4 CIDRs", () => {
    expect(isIpv4Cidr("10.40.0.0/16")).toBe(true);
    expect(isIpv4Cidr("0.0.0.0/0")).toBe(true);
    expect(isIpv4Cidr("10.40.0.0")).toBe(false);
    expect(isIpv4Cidr("256.0.0.0/8")).toBe(false);
    expect(isIpv4Cidr("10.0.0.0/33")).toBe(false);
  });
});
