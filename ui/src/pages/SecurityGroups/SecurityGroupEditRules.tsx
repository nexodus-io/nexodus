import React, { useEffect, useState } from "react";
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
import * as Mui from "@mui/material";
import Autocomplete from "@mui/material/Autocomplete";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";
import {
  ProtocolAliases,
  SecurityGroup,
  SecurityRule,
  UpdateSecurityGroup,
} from "./SecurityGroupStructs";
import { fetchJson, backend } from "../../common/Api";
import Notifications from "../../common/Notifications";

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

  const [ipRangeInputValue, setIpRangeInputValue] = useState<string[]>([]);

  useEffect(() => {
    // Initialize tempPortValues whenever secRule changes
    const initialPortValues = secRule.map(
      (rule) =>
        `${rule.from_port}${
          rule.to_port !== rule.from_port ? `-${rule.to_port}` : ""
        }`,
    );
    setTempPortValues(initialPortValues);
  }, [secRule]);

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

  const validatePortRange = (
    port: number,
    type: string,
    index: number,
  ): string | null => {
    // TODO: Add back validate port logic in a separate PR. Tracked in issue #1445
    return null;
  };

  const [tempPortValues, setTempPortValues] = useState<string[]>([]);

  const handleProtocolChange = (e: Mui.SelectChangeEvent, index: number) => {
    const selectedProtocol = e.target.value;
    let updatedRule = { ...secRule[index], ip_protocol: selectedProtocol };

    if (ProtocolAliases[selectedProtocol]) {
      const { port, type } = ProtocolAliases[selectedProtocol];
      updatedRule = {
        ...updatedRule,
        from_port: port,
        to_port: port,
        ip_protocol: type,
      };
    } else if (["icmpv4", "icmpv6"].includes(selectedProtocol)) {
      updatedRule = { ...updatedRule, from_port: 0, to_port: 0 };
    }

    onRuleChange(index, updatedRule);
  };

  const getProtocolNameByPorts = (
    from_port: number,
    to_port: number,
    ip_protocol: string,
  ): string | null => {
    console.debug(
      `Looking for protocol with from_port: ${from_port}, to_port: ${to_port}, ip_protocol: ${ip_protocol}`,
    );
    // Loop through the ProtocolAliases to find a match, either a proto with a defined set
    // of ports e.g., SSH or a protocol such as TCP with a port of 0-0, should always match
    for (const [key, value] of Object.entries(ProtocolAliases)) {
      if (
        value.port === from_port &&
        value.port === to_port &&
        value.type === ip_protocol
      ) {
        console.log(`Found match: ${key}`);
        return key;
      }
    }
    return null;
  };

  const isPredefinedRule = (protocol: string): boolean => {
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
      "TCP",
      "UDP",
    ].includes(protocol);
  };

  const isUnmodifiableIpRange = (protocol: string): boolean => {
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
          <Tooltip
            title="Adding a rule will begin blocking all traffic not explicitly allowed by a rule for the traffic direction you are editing (inbound or outbound)"
            placement="top"
          >
            <Button onClick={() => {}}>
              <HelpOutlineIcon />
            </Button>
          </Tooltip>
          <Button onClick={handleSaveRules}>Save Rules</Button>
          <Tooltip
            title="Removing all rules will allow all traffic for the rule direction you are editing (inbound or outbound)"
            placement="top"
          >
            <Button onClick={() => {}}>
              <HelpOutlineIcon />
            </Button>
          </Tooltip>
        </ButtonGroup>
      </div>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell style={{ fontSize: "14", width: "30%" }}>
              IP Protocol
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "20%" }}>
              <div style={{ display: "flex", alignItems: "center" }}>
                Port Range
                <Tooltip
                  title="Add a port number or a comma-seperated port range between 1-65535. A value of 0 opens all ports."
                  placement="top"
                >
                  <HelpOutlineIcon
                    fontSize="small"
                    style={{ marginLeft: "8px" }}
                  />
                </Tooltip>
              </div>
            </TableCell>
            <TableCell style={{ fontSize: "14", width: "40%" }}>
              <div style={{ display: "flex", alignItems: "center" }}>
                IP Ranges
                <Tooltip
                  title="Add a an IP range in the form of an IP address, an IP CIDR or a comma separated address range and then hit return to populate the list"
                  placement="top"
                >
                  <HelpOutlineIcon
                    fontSize="small"
                    style={{ marginLeft: "8px" }}
                  />
                </Tooltip>
              </div>
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
                      (getProtocolNameByPorts(
                        rule.from_port,
                        rule.to_port,
                        rule.ip_protocol,
                      ) as any) || (rule.ip_protocol as any)
                    }
                    onChange={(e: Mui.SelectChangeEvent) =>
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
                {/* Port Range Column*/}
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <TextField
                    value={tempPortValues[index] || ""}
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
                {/*IP Ranges Column*/}
                <TableCell
                  style={{
                    paddingBottom: hasErrorInRow(index) ? "2em" : undefined,
                  }}
                >
                  <Autocomplete
                    multiple // allow multiple prefixes in the cell
                    inputValue={ipRangeInputValue[index] || ""} // Use index to manage individual rows
                    onInputChange={(_, newInputValue) => {
                      console.debug(
                        "onInputChange - newInputValue:",
                        newInputValue,
                      );
                      // Update state for the individual row
                      setIpRangeInputValue((prev) => {
                        const newArr = [...prev];
                        newArr[index] = newInputValue;
                        return newArr;
                      });
                    }}
                    disabled={isUnmodifiableIpRange(rule.ip_protocol)}
                    options={[
                      "::/0",
                      "0.0.0.0/0",
                      {
                        title: "Organization IPv4",
                        value: "100.64.0.0/10",
                      },
                      { title: "Organization IPv6", value: "0200::/8" },
                    ]}
                    getOptionLabel={(option) =>
                      typeof option === "string" ? option : option.title
                    }
                    getOptionDisabled={(option) =>
                      isUnmodifiableIpRange(rule.ip_protocol)
                    }
                    value={rule.ip_ranges || []}
                    freeSolo
                    autoHighlight // Highlight the first match as the user types
                    onChange={(_, newValue) => {
                      console.debug("onChange - newValue:", newValue);
                      if (isUnmodifiableIpRange(rule.ip_protocol)) {
                        const updatedRule = { ...rule, ip_ranges: [] };
                        onRuleChange(index, updatedRule);
                        setIpRangeInputValue((prev) => {
                          const newArr = [...prev];
                          newArr[index] = "";
                          return newArr;
                        });
                        return;
                      }
                      const updatedIpRanges = newValue.map((item) =>
                        typeof item === "string" || typeof item === "number"
                          ? item
                          : item.value,
                      );
                      const updatedRule = {
                        ...rule,
                        ip_ranges: updatedIpRanges,
                      };
                      onRuleChange(index, updatedRule);
                      setIpRangeInputValue((prev) => {
                        const newArr = [...prev];
                        newArr[index] = "";
                        return newArr;
                      }); // Clear the input value
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

                {/*Actions Column*/}
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
