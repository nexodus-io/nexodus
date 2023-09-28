import {
  Button,
  ButtonGroup,
  Table,
  TableHead,
  TableRow,
  TableBody,
  TableCell,
  TextField,
  Select,
  MenuItem,
  Tooltip,
} from "@mui/material";
import {
  IpProtocol,
  SecurityGroup,
  SecurityRule,
  UpdateSecurityGroup,
} from "./SecurityGroupStructs";
import { fetchJson, backend } from "../../common/Api";
import React, { useEffect, useState } from "react";
import Notifications from "../../common/Notifications";
import * as Mui from "@mui/material";
import Autocomplete from "@mui/material/Autocomplete";
import { validateProtocolAndIpRange } from "../../common/IpHelpers";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";

interface EditRulesProps {
  groupName: string;
  groupDescription: string;
  secRule: SecurityRule[];
  data: SecurityGroup;
  inboundRules: SecurityRule[];
  outboundRules: SecurityRule[];
  // Update any rules changes if there are any
  onRuleChange: (index: number, updatedRule: SecurityRule) => void;
  organizationId: string | null;
  securityGroupId: string | null;
  // Callback to parent to update rules
  updateData?: (
    type: "inbound_rules" | "outbound_rules",
    updatedRules: SecurityRule[],
  ) => void;
  type: "inbound_rules" | "outbound_rules";
}

const updateSecurityGroup = (
  organizationId: string,
  securityGroupId: string,
  data: any,
) => {
  console.log("Update SecGroup data being sent to the endpoint:", data);
  console.log(
    "Sending PATCH request to:",
    `${backend}/api/organizations/${organizationId}/security_groups/${securityGroupId}`,
  );
  console.log("Data being sent:", JSON.stringify(data, null, 2));

  const requestOptions = {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
    credentials: "include",
  };

  return fetchJson(
    `${backend}/api/organizations/${organizationId}/security_groups/${securityGroupId}`,
    requestOptions,
  );
};

const EditRules: React.FC<EditRulesProps> = ({
  onRuleChange,
  groupName,
  groupDescription,
  inboundRules,
  outboundRules,
  secRule,
  organizationId,
  securityGroupId,
  updateData,
  type,
  data,
}) => {
  // Snackbar notifications in common/Notifications.tsx
  const [notificationMessage, setNotificationMessage] = useState<string | null>(
    null,
  );
  const [notificationType, setNotificationType] = useState<
    "success" | "error" | "info" | null
  >(null);

  // Message box errors for table edits
  const [fieldErrors, setFieldErrors] = useState<
    { from_port?: string; to_port?: string }[]
  >([]);

  const derivedRules =
    type === "inbound_rules"
      ? data.inbound_rules || []
      : data.outbound_rules || [];
  const [rules, setRules] = useState<SecurityRule[]>(derivedRules);

  const handleSaveRules = () => {
    let valid = true;
    const errorMessages: {
      index: number;
      fromPortValidation: string;
      toPortValidation: string;
    }[] = [];

    // Loop through the rules to validate the port ranges
    rules.forEach((rule, index) => {
      const fromPortValidation: string | null = validatePortRange(
        rule.from_port,
        "from_port",
        index,
      );
      const toPortValidation: string | null = validatePortRange(
        rule.to_port,
        "to_port",
        index,
      );

      if (fromPortValidation !== null || toPortValidation !== null) {
        valid = false;
        setNotificationType("error");
        setNotificationMessage("Failed to validate the ports");
      }
    });

    if (valid) {
      if (organizationId && securityGroupId) {
        const updateData: UpdateSecurityGroup = {
          group_name: groupName,
          group_description: groupDescription,
          inbound_rules: inboundRules,
          outbound_rules: outboundRules,
        };

        updateSecurityGroup(organizationId, securityGroupId, updateData)
          .then(() => {
            // Handle the response
            setNotificationType("success");
            setNotificationMessage("Rules saved successfully");
          })
          .catch((error) => {
            setNotificationType("error");
            setNotificationMessage("Failed to save rules");
            console.error("Error updating security group:", error);
          });
      } else {
        console.error("Organization ID or Security Group ID is missing.");
      }
    } else {
      // update fieldErrors state here?
      console.error("Validation failed:", errorMessages);
    }
  };

  const handleAddRule = () => {
    console.log("Add rule from Edit Rules called");
    const newRule: SecurityRule = {
      ip_protocol: "",
      from_port: 0,
      to_port: 0,
      ip_ranges: [],
    };
    const newRules = [...secRule, newRule];
    setRules(newRules);
    updateData && updateData(type, newRules);
  };

  const handleDeleteRule = (index: number) => {
    const newRules = [...secRule];
    newRules.splice(index, 1);
    setRules(newRules);
    updateData && updateData(type, newRules);
  };

  // TODO: Implement for v4/v6 sanity checks against protocol and family. Tracked in issue #1445
  const handleIpRangeChange = (newValue: string[], index: number) => {
    const updatedRule = {
      ...secRule[index],
      ip_ranges: newValue,
    };

    try {
      validateProtocolAndIpRange(updatedRule.ip_protocol, newValue);
    } catch (error: any) {
      if (error instanceof Error) {
        setNotificationType("error");
        setNotificationMessage(error.message);
      }
      return; // Skip updating the rule if validation fails
    }

    onRuleChange(index, updatedRule);
  };

  const validatePortRange = (
    port: number,
    type: string,
    index: number,
  ): string | null => {
    // TODO: Add back validate port logic in a separate PR. Tracked in issue #1445
    return null;
  };

  const [tempPortValues, setTempPortValues] = useState<string[]>([]);

  const handleProtocolChange = (
    e: Mui.SelectChangeEvent<IpProtocol>,
    index: number,
  ) => {
    const aliasProtocol = e.target.value as IpProtocol;
    let updatedRule = { ...secRule[index], ip_protocol: aliasProtocol };
    switch (aliasProtocol) {
      case "SSH":
        updatedRule = {
          ...updatedRule,
          from_port: 22,
          to_port: 22,
          ip_protocol: "tcp",
        };
        break;
      case "HTTP":
        updatedRule = {
          ...updatedRule,
          from_port: 80,
          to_port: 80,
          ip_protocol: "tcp",
        };
        break;
      case "HTTPS":
        updatedRule = {
          ...updatedRule,
          from_port: 443,
          to_port: 443,
          ip_protocol: "tcp",
        };
        break;
      case "PostgreSQL":
        updatedRule = {
          ...updatedRule,
          from_port: 5432,
          to_port: 5432,
          ip_protocol: "tcp",
        };
        break;
      case "MySQL":
        updatedRule = {
          ...updatedRule,
          from_port: 3306,
          to_port: 3306,
          ip_protocol: "tcp",
        };
        break;
      case "SMB":
        updatedRule = {
          ...updatedRule,
          from_port: 445,
          to_port: 445,
          ip_protocol: "tcp",
        };
        break;
      case "icmpv4":
      case "icmpv6":
        updatedRule = { ...updatedRule, from_port: 0, to_port: 0 };
        break;
      case "icmp": // ALL ICMP
        updatedRule = {
          ...updatedRule,
          from_port: 0,
          to_port: 0,
          ip_ranges: [],
        };
        break;
    }

    onRuleChange(index, updatedRule);
  };

  const isPredefinedRule = (protocol: IpProtocol): boolean => {
    return [
      "SSH",
      "HTTP",
      "HTTPS",
      "PostgreSQL",
      "MySQL",
      "SMB",
      "icmp",
      "icmpv4",
      "icmpv6",
      "ip",
    ].includes(protocol);
  };

  // map predefined protocol names to their corresponding port ranges for display render only, rules get sent with ip_protocol:tcp
  const getPredefinedProtocolName = (
    from_port: number,
    to_port: number,
  ): IpProtocol | undefined => {
    const predefinedProtocols: { [key: string]: IpProtocol } = {
      "22-22": "SSH",
      "80-80": "HTTP",
      "443-443": "HTTPS",
      "5432-5432": "PostgreSQL",
      "3306-3306": "MySQL",
      "445-445": "SMB",
    };
    return predefinedProtocols[`${from_port}-${to_port}`];
  };

  const isUnmodifiableIpRange = (protocol: IpProtocol): boolean => {
    return ["ip", "icmp"].includes(protocol);
  };

  const hasErrorInRow = (index: number): boolean => {
    return !!fieldErrors[index]?.from_port || !!fieldErrors[index]?.to_port;
  };

  return (
    <>
      <div style={{ marginTop: "20px", marginBottom: "10px" }}>
        <ButtonGroup variant="outlined" style={{ marginRight: "10px" }}>
          <Button onClick={handleAddRule}>Add Rule</Button>
          <Tooltip title="Adding a rule will begin blocking all traffic not explicitly allowed by a rule for the traffic direction you are editing (inbound or outbound)">
            <Button onClick={() => {}}>
              <HelpOutlineIcon />
            </Button>
          </Tooltip>
          <Button onClick={handleSaveRules}>Save Rules</Button>
          <Tooltip title="Removing all rules will allow all traffic for the rule direction you are editing (inbound or outbound)">
            <Button onClick={() => {}}>
              <HelpOutlineIcon />
            </Button>
          </Tooltip>
        </ButtonGroup>
      </div>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell style={{ fontSize: "14", width: "20%" }}>
              IP Protocol
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "10%" }}>
              Port Range
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "25%" }}>
              IP Ranges
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "25%" }}>
              Defined IP Ranges
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "10%" }}>
              Action
            </TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {secRule ? (
            secRule.map((rule, index) => (
              <TableRow
                key={index}
                style={{ paddingTop: "40", paddingBottom: "40" }}
              >
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <Select
                    value={
                      (getPredefinedProtocolName(
                        rule.from_port,
                        rule.to_port,
                      ) as any) || (rule.ip_protocol as any)
                    }
                    onChange={(e: Mui.SelectChangeEvent<IpProtocol>) =>
                      handleProtocolChange(e, index)
                    }
                    variant="outlined"
                    size="small"
                    fullWidth
                  >
                    <MenuItem value="">Select Option</MenuItem>
                    <MenuItem value="icmp">All ICMP</MenuItem>
                    <MenuItem value="tcp">TCP</MenuItem>
                    <MenuItem value="udp">UDP</MenuItem>
                    <MenuItem value="icmpv6">ICMPv6</MenuItem>
                    <MenuItem value="ipv6">IPv6</MenuItem>
                    <MenuItem value="ipv4">IPv4</MenuItem>
                    <MenuItem value="icmpv4">ICMPv4</MenuItem>
                    <MenuItem value="SSH">SSH</MenuItem>
                    <MenuItem value="HTTP">HTTP</MenuItem>
                    <MenuItem value="HTTPS">HTTPS</MenuItem>
                    <MenuItem value="PostgreSQL">PostgreSQL</MenuItem>
                    <MenuItem value="MySQL">MySQL</MenuItem>
                    <MenuItem value="SMB">SMB</MenuItem>
                  </Select>
                </TableCell>
                {/* From Port - Starting Port Range  */}
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <TextField
                    value={
                      isPredefinedRule(
                        getPredefinedProtocolName(
                          rule.from_port,
                          rule.to_port,
                        ) || rule.ip_protocol,
                      )
                        ? `${rule.from_port}${
                            rule.to_port !== rule.from_port
                              ? `-${rule.to_port}`
                              : ""
                          }`
                        : tempPortValues[index] || ""
                    }
                    onFocus={() => {
                      const newTempPortValues = [...tempPortValues];
                      newTempPortValues[index] = `${rule.from_port}${
                        rule.to_port !== rule.from_port
                          ? `-${rule.to_port}`
                          : ""
                      }`;
                      setTempPortValues(newTempPortValues);
                    }}
                    onChange={(e) => {
                      const newTempPortValues = [...tempPortValues];
                      newTempPortValues[index] = e.target.value;
                      setTempPortValues(newTempPortValues);
                    }}
                    onBlur={() => {
                      let [from_port, to_port] = tempPortValues[index]
                        .split("-")
                        .map(Number);
                      if (isNaN(to_port)) {
                        to_port = from_port;
                      }
                      const updatedRule = {
                        ...rule,
                        from_port,
                        to_port,
                      };
                      onRuleChange(index, updatedRule);
                    }}
                    placeholder="Ports"
                    type="text"
                    variant="outlined"
                    size="small"
                    fullWidth
                    style={{
                      // TODO: this doesnt work for aliased PROTOs, e.g., mysql, https, etc.
                      backgroundColor: isPredefinedRule(rule.ip_protocol)
                        ? "#E8F4F9"
                        : "transparent",
                    }}
                    disabled={isPredefinedRule(rule.ip_protocol)}
                  />
                </TableCell>
                {/*User Defined IP Ranges*/}
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <TextField
                    variant="outlined"
                    size="small"
                    fullWidth
                    value={rule.ip_ranges?.join(", ") ?? ""}
                    onChange={(e) => {
                      const newValue = e.target.value
                        .split(",")
                        .map((item) => item.trim());
                      const updatedRule = { ...rule, ip_ranges: newValue };
                      onRuleChange(index, updatedRule);
                    }}
                    style={{
                      backgroundColor: isUnmodifiableIpRange(rule.ip_protocol)
                        ? "#E8F4F9"
                        : "transparent",
                    }}
                    disabled={isUnmodifiableIpRange(rule.ip_protocol)}
                  />
                </TableCell>
                {/*PreDefined IP Ranges*/}
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <Autocomplete
                    multiple
                    // Disable the dropdown based on the condition such as "All ICMP"
                    disabled={isUnmodifiableIpRange(rule.ip_protocol)}
                    options={[
                      "::/0",
                      "0.0.0.0/0",
                      {
                        title: "Nexodus Private IPv4 CIDR",
                        value: "100.64.0.0/10",
                      },
                      { title: "Nexodus Private IPv6 CIDR", value: "0200::/8" },
                    ]}
                    getOptionLabel={(option) =>
                      typeof option === "string" ? option : option.title
                    }
                    getOptionDisabled={(option) =>
                      isUnmodifiableIpRange(rule.ip_protocol)
                    }
                    value={rule.ip_ranges || []}
                    onChange={(_, newValue) => {
                      const updatedRule = {
                        ...rule,
                        ip_ranges: newValue.map((item) =>
                          typeof item === "string" ? item : item.value,
                        ),
                      };
                      onRuleChange(index, updatedRule);
                    }}
                    renderInput={(params) => (
                      <TextField
                        {...params}
                        variant="outlined"
                        size="small"
                        fullWidth
                        style={{
                          backgroundColor: isUnmodifiableIpRange(
                            rule.ip_protocol,
                          )
                            ? "#E8F4F9"
                            : "transparent",
                        }}
                        disabled={isUnmodifiableIpRange(rule.ip_protocol)}
                      />
                    )}
                  />
                </TableCell>
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <Button onClick={() => handleDeleteRule(index)}>
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={6}>No rules found</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
      <Notifications message={notificationMessage} type={notificationType} />
    </>
  );
};

export default EditRules;
