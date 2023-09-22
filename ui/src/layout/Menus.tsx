import { DashboardMenuItem, MenuItemLink, Menu } from "react-admin";
import SecurityIcon from "@mui/icons-material/Security";
import UserIcon from "@mui/icons-material/People";
import DeviceIcon from "@mui/icons-material/Devices";
import OrganizationIcon from "@mui/icons-material/VpnLock";
import InvitationIcon from "@mui/icons-material/Rsvp";
import { MenuProps } from "react-admin";

export const CustomMenu = (props: MenuProps) => {
  return (
    <Menu {...props}>
      <DashboardMenuItem />
      <MenuItemLink to="/users" primaryText="Users" leftIcon={<UserIcon />} />
      <MenuItemLink
        to="/organizations"
        primaryText="Organizations"
        leftIcon={<OrganizationIcon />}
      />
      <MenuItemLink
        to="/devices"
        primaryText="Devices"
        leftIcon={<DeviceIcon />}
      />
      <MenuItemLink
        to="/invitations"
        primaryText="Invitations"
        leftIcon={<InvitationIcon />}
      />
      <MenuItemLink
        to="/security-groups"
        primaryText="SecurityGroups"
        leftIcon={<SecurityIcon />}
      />
    </Menu>
  );
};
