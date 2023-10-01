import React, { useEffect, useState } from "react";
import {
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { SecurityGroup, SecurityRule } from "./SecurityGroupStructs";

interface Props {
  data: SecurityGroup;
  type: "inbound_rules" | "outbound_rules";
  editable?: boolean;
  // callback to parent to update rules
  updateData?: (
    type: "inbound_rules" | "outbound_rules",
    updatedRules: SecurityRule[],
  ) => void;
}

const SecurityGroupTable: React.FC<Props> = ({
  data,
  type,
  editable = false,
  updateData,
}) => {
  const derivedRules =
    type === "inbound_rules"
      ? data.inbound_rules || []
      : data.outbound_rules || [];

  const [rules, setRules] = useState<SecurityRule[]>(derivedRules);

  useEffect(() => {
    console.log("Setting rules in useEffect:", derivedRules);
    if (!rules.length) {
      // Only update if the rules are empty (TODO: not sure this is needed)
      setRules(derivedRules);
    }
  }, [type, data]);

  const handleDeleteRule = (index: number) => {
    const newRules = [...rules];
    newRules.splice(index, 1);
    setRules(newRules);
    updateData && updateData(type, newRules);
  };

  const handleRuleChange = (
    index: number,
    field: keyof SecurityRule,
    value: any,
  ) => {
    const newRules = [...rules];

    if (field === "ip_protocol" && typeof value === "string") {
      newRules[index].ip_protocol = value;
    } else if (
      (field === "from_port" || field === "to_port") &&
      typeof value === "number"
    ) {
      newRules[index][field] = value;
    } else if (field === "ip_ranges" && Array.isArray(value)) {
      newRules[index].ip_ranges = value;
    }

    setRules(newRules);
    updateData && updateData(type, newRules);
  };

  return (
    <Paper>
      <Typography variant="body1" style={{ padding: "16px" }}>
        {type.replace("_", " ").toUpperCase()}
      </Typography>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell>IP Protocol</TableCell>
            <TableCell>Port Ranges</TableCell>
            <TableCell>IP Ranges</TableCell>
            {editable && <TableCell>Action</TableCell>}
          </TableRow>
        </TableHead>
        <TableBody>
          {Array.isArray(rules) &&
            rules.map((rule, index) => (
              <TableRow key={index}>
                <TableCell>
                  {editable ? (
                    <TextField
                      value={rule.ip_protocol}
                      onChange={(e) =>
                        handleRuleChange(index, "ip_protocol", e.target.value)
                      }
                    />
                  ) : (
                    rule.ip_protocol
                  )}
                </TableCell>
                <TableCell>
                  {rule.from_port === rule.to_port
                    ? rule.from_port
                    : `${rule.from_port}-${rule.to_port}`}
                </TableCell>
                <TableCell>
                  {editable ? (
                    <TextField
                      value={rule.ip_ranges?.join(", ") || ""}
                      onChange={(e) =>
                        handleRuleChange(
                          index,
                          "ip_ranges",
                          e.target.value.split(",").map((s) => s.trim()),
                        )
                      }
                    />
                  ) : (
                    rule.ip_ranges?.join(", ") || ""
                  )}
                </TableCell>
                {editable && (
                  <TableCell>
                    <IconButton onClick={() => handleDeleteRule(index)}>
                      <DeleteIcon />
                    </IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))}
        </TableBody>
      </Table>
    </Paper>
  );
};

export default SecurityGroupTable;
