import React, { useState, useEffect } from "react";
import SecurityGroupTable from "./SecurityGroupTable";
import RefreshIcon from "@mui/icons-material/Refresh";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tabs,
  Tab,
  Button,
} from "@mui/material";
import { SecurityGroup, SecurityRule } from "./SecurityGroupStructs";
import EditRules from "./SecurityGroupEditRules";
import { backend, fetchJson } from "../../common/Api";
import Notifications from "../../common/Notifications";

export const SecurityGroups = () => {
  const [securityGroups, setSecurityGroups] = useState<SecurityGroup[]>([]);
  const [selectedSecurityGroup, setSelectedSecurityGroup] =
    useState<SecurityGroup | null>(null);

  const [activeTab, setActiveTab] = useState<
    "inbound_rules" | "outbound_rules"
  >("inbound_rules");
  const [isEditing, setIsEditing] = useState(false);
  const [editedRules, setEditedRules] = useState<SecurityRule[]>([]);
  const [inboundRules, setInboundRules] = useState<SecurityRule[]>([]);
  const [outboundRules, setOutboundRules] = useState<SecurityRule[]>([]);

  // Snackbar notifications in common/Notifications.tsx
  const [notificationMessage, setNotificationMessage] = useState<string | null>(
    null,
  );
  const [notificationType, setNotificationType] = useState<
    "success" | "error" | "info" | null
  >(null);

  const fetchSecurityGroup = async (securityGroupId: string) => {
    return await fetchJson(`${backend}/api/security-groups/${securityGroupId}`);
  };

  const fetchSecurityGroups = async () => {
    return await fetchJson(`${backend}/api/security-groups`);
  };

  const fetchData = async () => {
    try {
      const securityGroupsData: SecurityGroup[] = await fetchSecurityGroups();
      setSecurityGroups(securityGroupsData);
    } catch (error) {
      console.error("Error:", error);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  // When a security group is selected, set those rules to your state
  useEffect(() => {
    if (selectedSecurityGroup) {
      setInboundRules(selectedSecurityGroup.inbound_rules);
      setOutboundRules(selectedSecurityGroup.outbound_rules);
    }
  }, [selectedSecurityGroup]);

  // When the cancel button is clicked to exit rules editing the updated rules need to be re-rendered.
  const handleExitEditMode = async () => {
    // Reset notification states
    setNotificationMessage(null);
    setNotificationType(null);
    setIsEditing(false);

    if (selectedSecurityGroup && selectedSecurityGroup.id) {
      try {
        const updatedSecurityGroupData = await fetchSecurityGroup(
          selectedSecurityGroup.id,
        );
        setSelectedSecurityGroup(updatedSecurityGroupData);
      } catch (error) {
        console.error("Error:", error);
        setNotificationType("error");
        setNotificationMessage(
          "Unable to fetch data from the Nexodus api-server.",
        );
      }
    } else {
      console.error("Error: Security Group ID is missing.");
      setNotificationType("error");
      setNotificationMessage("Security Group ID is missing.");
    }
  };

  const handleSelectSecurityGroup = (securityGroup: SecurityGroup) => {
    setSelectedSecurityGroup(securityGroup);
  };

  const handleEditClick = () => {
    setIsEditing(true);
    const rulesToEdit =
      activeTab === "inbound_rules"
        ? selectedSecurityGroup?.inbound_rules || []
        : selectedSecurityGroup?.outbound_rules || [];
    console.log("Setting editedRules:", rulesToEdit);
    setEditedRules(rulesToEdit);
  };

  const handleTabChange = (
    event: React.ChangeEvent<{}>,
    newValue: "inbound_rules" | "outbound_rules",
  ) => {
    setActiveTab(newValue);
  };

  const updateDataInParent = (
    type: "inbound_rules" | "outbound_rules",
    updatedRules: SecurityRule[],
  ) => {
    if (type === "inbound_rules") {
      setInboundRules(updatedRules);
    } else {
      setOutboundRules(updatedRules);
    }
  };

  return (
    <div>
      {/* Listen for notification state */}
      <Notifications type={notificationType} message={notificationMessage} />
      <Table>
        <TableHead>
          <TableRow>
            <TableCell>Security Group ID</TableCell>
            <TableCell>Security Group Description</TableCell>
            <TableCell>VPC ID</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {securityGroups.map((securityGroup) => (
            <TableRow
              key={securityGroup.id}
              onClick={() => handleSelectSecurityGroup(securityGroup)}
              style={{ cursor: "pointer" }}
            >
              <TableCell>{securityGroup.id}</TableCell>
              <TableCell>{securityGroup.description}</TableCell>
              <TableCell>{securityGroup.vpc_id}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {selectedSecurityGroup ? (
        <>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              paddingTop: "40px",
              marginBottom: "20px",
            }}
          >
            <div style={{ display: "flex", alignItems: "center" }}>
              <Tabs
                value={activeTab}
                onChange={handleTabChange}
                style={{ marginRight: "10px" }}
              >
                <Tab label="Inbound Rules" value="inbound_rules" />
                <Tab label="Outbound Rules" value="outbound_rules" />
              </Tabs>
              {isEditing ? (
                <>
                  <Button variant="outlined" onClick={handleExitEditMode}>
                    Cancel
                  </Button>
                </>
              ) : (
                <Button variant="outlined" onClick={handleEditClick}>
                  Edit Rules
                </Button>
              )}
            </div>
            <Button onClick={fetchData}>
              <RefreshIcon />
            </Button>
          </div>
          {isEditing ? (
            <EditRules
              secRule={
                activeTab === "inbound_rules"
                  ? inboundRules || []
                  : outboundRules || []
              }
              onRuleChange={(index, updatedRule) => {
                if (activeTab === "inbound_rules") {
                  setInboundRules((prev) => {
                    const updatedRules = [...prev];
                    updatedRules[index] = updatedRule;
                    return updatedRules;
                  });
                } else {
                  setOutboundRules((prev) => {
                    const updatedRules = [...prev];
                    updatedRules[index] = updatedRule;
                    return updatedRules;
                  });
                }
              }}
              groupDescription={selectedSecurityGroup?.description || ""}
              inboundRules={inboundRules}
              outboundRules={outboundRules}
              securityGroupId={selectedSecurityGroup?.id || null}
              key={activeTab}
              data={selectedSecurityGroup}
              type={activeTab}
              updateData={updateDataInParent}
            />
          ) : (
            <SecurityGroupTable
              key={activeTab}
              data={selectedSecurityGroup}
              type={activeTab}
              updateData={updateDataInParent}
              inboundRules={inboundRules}
              outboundRules={outboundRules}
            />
          )}
        </>
      ) : (
        <p></p>
      )}
    </div>
  );
};

export default SecurityGroups;
