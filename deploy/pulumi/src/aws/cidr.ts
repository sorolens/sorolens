/** Minimal IPv4 equivalent of Terraform's cidrsubnet(prefix, newbits, netnum). */
export function cidrSubnet(prefix: string, newBits: number, netNum: number): string {
  const [addr, lenText] = prefix.split("/");
  const len = Number(lenText);
  const octets = (addr ?? "").split(".").map(Number);
  if (octets.length !== 4 || octets.some((o) => !Number.isInteger(o) || o < 0 || o > 255) || !Number.isInteger(len)) {
    throw new Error(`invalid IPv4 CIDR "${prefix}"`);
  }
  const newLen = len + newBits;
  if (newLen > 32 || netNum < 0 || netNum >= 2 ** newBits) {
    throw new Error(`cannot take subnet ${netNum} of ${prefix} with ${newBits} new bits`);
  }
  const base = octets.reduce((acc, o) => acc * 256 + o, 0);
  const mask = len === 0 ? 0 : (0xffffffff << (32 - len)) >>> 0;
  const network = (base & mask) >>> 0;
  const subnet = network + netNum * 2 ** (32 - newLen);
  const parts = [24, 16, 8, 0].map((shift) => Math.floor(subnet / 2 ** shift) % 256);
  return `${parts.join(".")}/${newLen}`;
}

/** True for a valid IPv4 CIDR such as 10.0.0.0/16. */
export function isIpv4Cidr(value: string): boolean {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/.exec(value);
  if (!m) return false;
  const octets = m.slice(1, 5).map(Number);
  const len = Number(m[5]);
  return octets.every((o) => o <= 255) && len <= 32;
}
