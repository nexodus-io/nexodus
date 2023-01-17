import {
  defaultTheme,
  Layout,
  AppBar,
  AppBarProps,
  ToggleThemeButton,
  LayoutProps,
} from "react-admin";
import {
  createTheme,
  Box,
  Theme,
  Typography,
  useMediaQuery,
} from "@mui/material";
import LogoSrc from "../logo.svg";

const darkTheme = createTheme({
  palette: { mode: "dark" },
});

const CustomAppBar = (props: JSX.IntrinsicAttributes & AppBarProps) => {
  const isLargeEnough = useMediaQuery<Theme>((theme) =>
    theme.breakpoints.up("sm")
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
      {isLargeEnough && <img src={LogoSrc} alt="Apex" height="40px" />}
      {isLargeEnough && <Box component="span" sx={{ flex: 1 }} />}
      <ToggleThemeButton lightTheme={defaultTheme} darkTheme={darkTheme} />
    </AppBar>
  );
};

export default CustomAppBar;
