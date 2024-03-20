import { DashboardMenuItem, Menu, MenuItemLink, MenuProps } from "react-admin";
import SecurityIcon from "@mui/icons-material/Security";
import DeviceIcon from "@mui/icons-material/Devices";
import SiteIcon from "@mui/icons-material/BorderOuter";
import OrganizationIcon from "@mui/icons-material/People";
import InvitationIcon from "@mui/icons-material/Rsvp";
import RegKeyIcon from "@mui/icons-material/Key";
import VPCIcon from "@mui/icons-material/Cloud";
import ServiceNetworkIcon from "@mui/icons-material/Cloud";
import HubIcon from '@mui/icons-material/Hub';
import { useFlags } from "../common/FlagsContext";

export const CustomMenu = (props: MenuProps) => {
  const flags = useFlags();

  return (
    <Menu {...props}>
      <DashboardMenuItem />
      <MenuItemLink
        to="/organizations"
        primaryText="Organizations"
        leftIcon={<OrganizationIcon />}
      />
      <MenuItemLink
        to="/invitations"
        primaryText="Invitations"
        leftIcon={<InvitationIcon />}
      />
      {flags["devices"] && (
        <MenuItemLink to="/vpcs" primaryText="VPCs" leftIcon={<VPCIcon />} />
      )}
      {flags["devices"] && (
        <MenuItemLink
          to="/devices"
          primaryText="Devices"
          leftIcon={<DeviceIcon />}
        />
      )}
      {flags["security-groups"] && (
        <MenuItemLink
          to="/_security-groups"
          primaryText="Security Groups"
          leftIcon={<SecurityIcon />}
        />
      )}
      {flags["sites"] && (
        <MenuItemLink
          to="/service-networks"
          primaryText="Service Networks"
          leftIcon={<ServiceNetworkIcon />}
        />
      )}
      {flags["sites"] && (
        <MenuItemLink to="/sites" primaryText="Sites" leftIcon={<SiteIcon />} />
      )}
      <MenuItemLink
        to="/reg-keys"
        primaryText="Registration Keys"
        leftIcon={<RegKeyIcon />}
      />
    </Menu>
  );
};
