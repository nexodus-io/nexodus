// Protocol aliases with predefined ports and types
export const ProtocolAliases: {
  [key: string]: { port: number; type: string };
} = {
  SSH: { port: 22, type: "tcp" },
  HTTP: { port: 80, type: "tcp" },
  HTTPS: { port: 443, type: "tcp" },
  PostgreSQL: { port: 5432, type: "tcp" },
  MySQL: { port: 3306, type: "tcp" },
  SMB: { port: 445, type: "tcp" },
  tcp: { port: 0, type: "tcp" },
  udp: { port: 0, type: "udp" },
  icmpv6: { port: 0, type: "icmpv6" },
  icmp: { port: 0, type: "icmp" },
  ipv6: { port: 0, type: "ipv6" },
  ipv4: { port: 0, type: "ipv4" },
  icmpv4: { port: 0, type: "icmpv4" },
};

interface SecurityRule {
  ip_ranges?: string[];
  to_port: number;
  from_port: number;
  ip_protocol: string;
}

// Represents a security group containing security rules and a group owner
interface SecurityGroup {
  id?: string;
  group_name: string;
  group_description: string;
  org_id: string; // UUID is a string
  inbound_rules: SecurityRule[];
  outbound_rules: SecurityRule[];
  revision: number;
}

// Represents the information needed to add a new Security Group.
interface AddSecurityGroup {
  group_name: string;
  group_description: string;
  org_id: string; // UUID is a string
  inbound_rules: SecurityRule[];
  outbound_rules: SecurityRule[];
}

// Represents the information needed to update an existing Security Group.
interface UpdateSecurityGroup {
  group_name?: string;
  group_description?: string;
  inbound_rules: SecurityRule[];
  outbound_rules: SecurityRule[];
}

interface Organization {
  id: string;
  name: string;
  security_group_id: string;
}

export type {
  SecurityGroup,
  AddSecurityGroup,
  UpdateSecurityGroup,
  SecurityRule,
  Organization,
};
