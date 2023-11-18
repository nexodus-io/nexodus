import { DashboardMenuItem, MenuItemLink, Menu } from "react-admin";
import SecurityIcon from "@mui/icons-material/Security";
import DeviceIcon from "@mui/icons-material/Devices";
import SiteIcon from "@mui/icons-material/BorderOuter";
import OrganizationIcon from "@mui/icons-material/People";
import InvitationIcon from "@mui/icons-material/Rsvp";
import { MenuProps } from "react-admin";
import RegKeyIcon from "@mui/icons-material/Key";
import VPCIcon from "@mui/icons-material/Cloud";

export const CustomMenu = (props: MenuProps) => {
  return (
    <Menu {...props}>
      <DashboardMenuItem />
      <MenuItemLink
        to="/organizations"
        primaryText="Organizations"
        leftIcon={<OrganizationIcon />}
      />
      <MenuItemLink to="/vpcs" primaryText="VPCs" leftIcon={<VPCIcon />} />
      <MenuItemLink
        to="/devices"
        primaryText="Devices"
        leftIcon={<DeviceIcon />}
      />
      <MenuItemLink to="/sites" primaryText="Sites" leftIcon={<SiteIcon />} />
      <MenuItemLink
        to="/invitations"
        primaryText="Invitations"
        leftIcon={<InvitationIcon />}
      />
      <MenuItemLink
        to="/_security-groups"
        primaryText="Security Groups"
        leftIcon={<SecurityIcon />}
      />
      <MenuItemLink
        to="/reg-keys"
        primaryText="Registration Keys"
        leftIcon={<RegKeyIcon />}
      />
    </Menu>
  );
};
