import * as React from 'react';
import { defaultTheme, Layout, AppBar, AppBarProps, ToggleThemeButton, LayoutProps } from 'react-admin';
import { createTheme, Box, Typography } from '@mui/material';
import { ReactQueryDevtools } from 'react-query/devtools';

const darkTheme = createTheme({
    palette: { mode: 'dark' },
});

const MyAppBar = (props: JSX.IntrinsicAttributes & AppBarProps) => (
    <AppBar {...props}>
        <Box flex="1">
            <Typography variant="h6" id="react-admin-title"></Typography>
        </Box>
        <ToggleThemeButton
            lightTheme={defaultTheme}
            darkTheme={darkTheme}
        />
    </AppBar>
);

export const MyLayout = (props: JSX.IntrinsicAttributes & LayoutProps) => (
    <>
    <Layout {...props} appBar={MyAppBar} />
    </>
);