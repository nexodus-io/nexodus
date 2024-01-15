import { AppBar, AppBarProps } from "react-admin";
import { Box, Theme, Typography, useMediaQuery } from "@mui/material";
import LogoSrc from "../logo.png";

const CustomAppBar = (props: JSX.IntrinsicAttributes & AppBarProps) => {
  const isLargeEnough = useMediaQuery<Theme>((theme) =>
    theme.breakpoints.up("sm"),
  );
  return (
    <AppBar
      sx={{
        "& .RaAppBar-title": {
          flex: 1,
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          overflow: "hidden",
        },
      }}
      elevation={1}
      {...props}
    >
      <Typography
        variant="h6"
        color="inherit"
        sx={{
          flex: 1,
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          overflow: "hidden",
        }}
        id="react-admin-title"
      ></Typography>
      {isLargeEnough && <img src={LogoSrc} alt="Nexodus" height="40px" />}
      {isLargeEnough && <Box component="span" sx={{ flex: 1 }} />}
    </AppBar>
  );
};

export default CustomAppBar;
