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
import {
  Organization,
  SecurityGroup,
  SecurityRule,
} from "./SecurityGroupStructs";
import EditRules from "./SecurityGroupEditRules";
import { backend, fetchJson } from "../../common/Api";
import Notifications from "../../common/Notifications";

export const SecurityGroups = () => {
  const [organizationId, setOrganizationId] = useState<string | null>(null);
  const [securityGroupId, setSecurityGroupId] = useState<string | null>(null);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [securityGroups, setSecurityGroups] = useState<SecurityGroup[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null);
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

  const fetchData = async () => {
    try {
      const orgs: Organization[] = await fetchJson(
        `${backend}/api/organizations`,
      );
      setOrgs(orgs);
      // Set the organizationId and securityGroupId based on the selected organization
      if (orgs.length > 0) {
        // TODO: handle an array of security groups
        setOrganizationId(orgs[0].id);
        setSecurityGroupId(orgs[0].security_group_id);
      }
      // Fetch security groups data
      const securityGroupPromises = orgs.map((org) =>
        fetchSecurityGroup(org.security_group_id),
      );

      const securityGroupsData = await Promise.all(securityGroupPromises);
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

  const handleExitEditMode = async () => {
    // Reset notification states
    setNotificationMessage(null);
    setNotificationType(null);
    setIsEditing(false);
    if (selectedOrg) {
      try {
        const updatedSecurityGroupData = await fetchSecurityGroup(
          selectedOrg.security_group_id,
        );
        setSelectedSecurityGroup(updatedSecurityGroupData);
      } catch (error) {
        console.error("Error:", error);
        setNotificationType("error");
        setNotificationMessage(
          "Unable to fetch data from the Nexodus api-server.",
        );
      }
    }
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

  const selectOrganization = (org: Organization) => {
    setSelectedOrg(org);
    // Updating state for the selected organization and security group
    setOrganizationId(org.id);
    setSecurityGroupId(org.security_group_id);

    const matchingSecurityGroup = securityGroups.find(
      (sg) => sg.id === org.security_group_id,
    );
    setSelectedSecurityGroup(matchingSecurityGroup || null);
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
            <TableCell>Security Group Name</TableCell>
            <TableCell>Security Group ID</TableCell>
            <TableCell>Security Group Description</TableCell>
            <TableCell>Organization Name</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {orgs.map((org, index) => {
            return (
              <TableRow
                key={org.id}
                onClick={() => selectOrganization(org)}
                style={{ cursor: "pointer" }}
              >
                <TableCell>{securityGroups[index]?.group_name}</TableCell>
                <TableCell>{org.security_group_id}</TableCell>
                <TableCell>
                  {securityGroups[index]?.group_description}
                </TableCell>
                <TableCell>{org.name}</TableCell>
              </TableRow>
            );
          })}
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
              groupName={selectedSecurityGroup?.group_name || ""}
              groupDescription={selectedSecurityGroup?.group_description || ""}
              inboundRules={inboundRules}
              outboundRules={outboundRules}
              organizationId={organizationId}
              securityGroupId={securityGroupId}
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
