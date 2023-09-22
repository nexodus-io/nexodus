import IPCIDR from "ip-cidr";

// Custom function to check for a valid IPv4 address since you can't use nodejs net module in a browser
const isIPv4 = (ip: string) => {
  const ipv4Regex =
    /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return ipv4Regex.test(ip);
};

// Custom function to check for a valid IPv6 address since you can't use nodejs net module in a browser
const isIPv6 = (ip: string) => {
  const ipv6Regex = /^(?:[A-Fa-f0-9]{1,4}:){7}[A-Fa-f0-9]{1,4}$/;
  return ipv6Regex.test(ip);
};

export const containsIPv4Range = (ipRanges: string[]): boolean => {
  for (const ipRange of ipRanges) {
    if (ipRange.includes("-")) {
      const ips = ipRange.split("-");
      if (isIPv4(ips[0]) && isIPv4(ips[1])) {
        return true;
      }
    } else if (IPCIDR.isValidCIDR(ipRange)) {
      const cidr = new IPCIDR(ipRange);
      const [startIp] = cidr.toRange();
      if (isIPv4(String(startIp))) {
        return true;
      }
    } else {
      if (isIPv4(ipRange)) {
        return true;
      }
    }
  }
  return false;
};

export const containsIPv6Range = (ipRanges: string[]): boolean => {
  for (const ipRange of ipRanges) {
    if (ipRange.includes("-")) {
      const ips = ipRange.split("-");
      if (isIPv6(ips[0]) && isIPv6(ips[1])) {
        return true;
      }
    } else if (IPCIDR.isValidCIDR(ipRange)) {
      const cidr = new IPCIDR(ipRange);
      const [startIp] = cidr.toRange();
      if (isIPv6(String(startIp))) {
        return true;
      }
    } else {
      if (isIPv6(ipRange)) {
        return true;
      }
    }
  }
  return false;
};

export const validateProtocolAndIpRange = (
  protocol: string,
  ipRanges: string[],
) => {
  if (protocol.toLowerCase().includes("v4") && containsIPv6Range(ipRanges)) {
    throw new Error("IPv6 range is not allowed for an IPv4 protocol selection");
  }
  if (protocol.toLowerCase().includes("v6") && containsIPv4Range(ipRanges)) {
    throw new Error("IPv4 range is not allowed for an IPv6 protocol selection");
  }
};
