import {
  Page,
  Button,
  Masthead,
  MastheadToggle,
  MastheadMain,
  MastheadBrand,
  MastheadContent,
  PageSidebar,
  PageSection,
  PageSectionVariants,
  PageToggleButton,
  Toolbar,
  ToolbarContent,
  ToolbarGroup,
  ToolbarItem,
} from '@patternfly/react-core';
import BarsIcon from '@patternfly/react-icons/dist/js/icons/bars-icon';
import OutlinedMoonIcon from '@patternfly/react-icons/dist/esm/icons/outlined-moon-icon';
import nexodusLogo from "../assets/wordmark_dark.png";
import React from 'react';
import { UserDropdown } from './UserDropdown';

export default function Root() {
  const [isNavOpen, setIsNavOpen] = React.useState(true);

  const onNavToggle = () => {
    setIsNavOpen(!isNavOpen);
  };

  const headerToolbar = (
    <Toolbar id="vertical-toolbar">
      <ToolbarContent alignment={{ default: "alignRight"}} >
          <ToolbarGroup variant="icon-button-group" alignment={{default: "alignRight"}}>
            <ToolbarItem>
            <Button variant="plain" aria-label="edit">
              <OutlinedMoonIcon />
            </Button>
            </ToolbarItem>
          </ToolbarGroup>  
        <ToolbarItem>
          <UserDropdown/>
        </ToolbarItem>
      </ToolbarContent>
    </Toolbar>
  );

  const header = (
    <Masthead>
      <MastheadToggle>
        <PageToggleButton
          variant="plain"
          aria-label="Global navigation"
          isNavOpen={isNavOpen}
          onNavToggle={onNavToggle}
          id="vertical-nav-toggle"
        >
          <BarsIcon />
        </PageToggleButton>
      </MastheadToggle>
      <MastheadMain>

        <MastheadBrand>
          <img src={nexodusLogo} alt="Nexodus Logo" width={100} />
        </MastheadBrand>

      </MastheadMain>
      <MastheadContent>{headerToolbar}</MastheadContent>
    </Masthead>
  );

  const sidebar = <PageSidebar nav="Navigation" isNavOpen={isNavOpen} id="vertical-sidebar" />;

  return (
    <Page header={header} sidebar={sidebar}>
      <PageSection variant={PageSectionVariants.darker}>Section with darker background</PageSection>
      <PageSection variant={PageSectionVariants.dark}>Section with dark background</PageSection>
      <PageSection variant={PageSectionVariants.light}>Section with light background</PageSection>
    </Page>
  );
}
