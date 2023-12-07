import { DashboardMenuItem, MenuItemLink, Menu } from "react-admin";
import SecurityIcon from "@mui/icons-material/Security";
import DeviceIcon from "@mui/icons-material/Devices";
import SiteIcon from "@mui/icons-material/BorderOuter";
import OrganizationIcon from "@mui/icons-material/People";
import InvitationIcon from "@mui/icons-material/Rsvp";
import { MenuProps } from "react-admin";
import RegKeyIcon from "@mui/icons-material/Key";
import VPCIcon from "@mui/icons-material/Cloud";
import { dataProvider } from "../DataProvider";
import { useEffect, useState } from "react";

export const CustomMenu = (props: MenuProps) => {
  const [flags, setFlags] = useState({} as { [index: string]: boolean });
  useEffect(() => {
    (async () => {
      try {
        setFlags(await dataProvider.getFlags());
      } catch (e) {
        console.log(e);
      }
    })();
  }, []);

  return (
    <Menu {...props}>
      <DashboardMenuItem />
      <MenuItemLink
        to="/organizations"
        primaryText="Organizations"
        leftIcon={<OrganizationIcon />}
        placeholder="" // Added placeholder
      />
      <MenuItemLink
        to="/vpcs"
        primaryText="VPCs"
        leftIcon={<VPCIcon />}
        placeholder=""
      />
      {flags["devices"] && (
        <MenuItemLink
          to="/devices"
          primaryText="Devices"
          leftIcon={<DeviceIcon />}
          placeholder=""
        />
      )}
      {flags["sites"] && (
        <MenuItemLink
          to="/sites"
          primaryText="Sites"
          leftIcon={<SiteIcon />}
          placeholder=""
        />
      )}
      <MenuItemLink
        to="/invitations"
        primaryText="Invitations"
        leftIcon={<InvitationIcon />}
        placeholder=""
      />
      {flags["security-groups"] && (
        <MenuItemLink
          to="/_security-groups"
          primaryText="Security Groups"
          leftIcon={<SecurityIcon />}
          placeholder=""
        />
      )}
      <MenuItemLink
        to="/reg-keys"
        primaryText="Registration Keys"
        leftIcon={<RegKeyIcon />}
        placeholder=""
      />
    </Menu>
  );
};
