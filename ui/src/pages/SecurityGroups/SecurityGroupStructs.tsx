// TODO: should probably be an enum / DRY etc
export type IpProtocol =
  | ""
  | "ip"
  | "tcp"
  | "udp"
  | "icmp"
  | "icmpv6"
  | "ipv6"
  | "ipv4"
  | "icmpv4"
  | "SSH"
  | "HTTP"
  | "HTTPS"
  | "PostgreSQL"
  | "MySQL"
  | "SMB";

// Represents a security rule
interface SecurityRule {
  ip_protocol: IpProtocol;
  from_port: number;
  to_port: number;
  ip_ranges?: string[];
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

export type {
  SecurityGroup,
  AddSecurityGroup,
  UpdateSecurityGroup,
  SecurityRule,
};
