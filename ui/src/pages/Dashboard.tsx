import React, { useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardMedia,
  Typography,
} from "@mui/material";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Paper from "@mui/material/TableRow";
import CloudIcon from '@mui/icons-material/Cloud';
import RouterIcon from "@mui/icons-material/Router";
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import { useTheme } from "@mui/material/styles";

import CardImage from "../wordmark.png";
import CardImageDark from "../wordmark_dark.png";
import { backend } from "../common/Api";

function createData(
  hostname: string,
  ipAddress: string,
  latency: number,
  peeringMethod: string,
  connectionStatus: string,
) {
  return { hostname, ipAddress, latency, peeringMethod, connectionStatus };
}

const deviceRows = [
  createData("80cbb9be04cc", "100.64.0.19", 25, "none", "Reachable"),
  createData(
    "48e5326fc084",
    "100.64.0.26",
    64,
    "relay-node-peer",
    "Unreachable",
  ),
  createData("194712efa971", "100.64.0.25", 47, "none", "Reachable"),
];

const relayRows = [
  createData(
    "48e5326fc084",
    "100.64.0.26",
    64,
    "relay-node-peer",
    "Unreachable",
  ),
  createData("80cbb9be04cc", "100.64.0.19", 25, "none", "Reachable"),
  createData("194712efa971", "100.64.0.25", 47, "none", "Reachable"),
];

const Dashboard: React.FC = () => {
  const theme = useTheme();

  const [isOpenDevice, setIsOpenDevice] = useState(false);
  const [isOpenRelay, setIsOpenRelay] = useState(false);
  const togglePopupDevice = () => {
    setIsOpenDevice(!isOpenDevice);
  };
  const togglePopupRelay = () => {
    setIsOpenRelay(!isOpenRelay);
  };

  return (
    <div>
      <Card
        raised
        sx={{
          margin: "0 auto",
          padding: "0.1em",
        }}
      >
        <CardMedia
          component="img"
          height="200"
          image={theme.palette?.mode === "dark" ? CardImageDark : CardImage}
          alt="nexodus banner"
          sx={{ padding: "1em 1em 0 1em", objectFit: "contain" }}
        />
        <CardHeader title="Welcome to Nexodus" />
        <CardContent>
          Test: Nexodus is a connectivity-as-a-service solution.
        </CardContent>
      </Card>
      <Card>
        <CardHeader title="Quick Start" />
        <CardContent>
          <Typography variant="body1">
            See the{" "}
            <a href="https://docs.nexodus.io/quickstart/">quick start</a> guide
            for instructions on how to get started.
          </Typography>
          <Typography variant="body1">
            See the <a href={backend + "/openapi/index.html"}>openapi</a>{" "}
            documentation to view the developer APIs.
          </Typography>
        </CardContent>
      </Card>
<<<<<<< HEAD
        <div style={{ display: 'flex', flexDirection: 'row' }}>
          <div style={{ marginRight: '50px' }}>
            <button style={{ ...styles.device,
            background: theme.palette.mode === 'dark' ? 'rgb(170, 170, 170)' : 'rgb(240, 240, 240)' }}
            onClick={togglePopupDevice}>
              <CloudIcon style={styles.deviceIcon} />
              Device
              <div style={styles.separator}></div>
              <span style={styles.ip}>100.64.0.19</span>
              <div style={styles.separator}></div>
              <OnlineIcon style={styles.onlineIcon} />
            </button>
            {isOpenDevice && (
                  <TableContainer component={Paper}>
                  <Table sx={{ minWidth: 600 }} aria-label="simple table">
                    <TableHead>
                      <TableRow>
                        <TableCell>Hostname</TableCell>
                        <TableCell align="center">IP Address</TableCell>
                        <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                        <TableCell align="center">Peering Method</TableCell>
                        <TableCell align="right">Connection Status</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {deviceRows.map((row, index) => (
                        <TableRow
                          key={row.hostname}
                          sx={{ '&:last-child td, &:last-child th': { border: 0 } }}
                        >
                          <TableCell component="th" scope="row" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>
                            {row.hostname}
                          </TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.ipAddress}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.latency}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.peeringMethod}</TableCell>
                          <TableCell align="right" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.connectionStatus}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
          </div>
          <div>
            <button style={{ ...styles.relay,
            background: theme.palette.mode === 'dark' ? 'rgb(170, 170, 170)' : 'rgb(240, 240, 240)' }}
            onClick={togglePopupRelay}>
              <RouterIcon style={styles.relayIcon} />
              Relay
              <div style={styles.separator}></div>
              <span style={styles.ip}>100.64.0.26</span>
              <div style={styles.separator}></div>
              <HighlightOffIcon style={styles.offlineIcon} />
            </button>
            {isOpenRelay && (
                  <TableContainer component={Paper}>
                  <Table sx={{ minWidth: 600 }} aria-label="simple table">
                    <TableHead>
                      <TableRow>
                        <TableCell>Hostname</TableCell>
                        <TableCell align="center">IP Address</TableCell>
                        <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                        <TableCell align="center">Peering Method</TableCell>
                        <TableCell align="right">Connection Status</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {relayRows.map((row, index) => (
                        <TableRow
                          key={row.hostname}
                          sx={{ '&:last-child td, &:last-child th': { border: 0 } }}
                        >
                          <TableCell component="th" scope="row" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>
                            {row.hostname}
                          </TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.ipAddress}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.latency}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.peeringMethod}</TableCell>
                          <TableCell align="right" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.connectionStatus}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
            )}
          </div>
=======
      <div style={{ display: "flex", flexDirection: "row" }}>
        <div style={{ marginRight: "50px" }}>
          <button
            style={{
              ...styles.device,
              background:
                theme.palette.mode === "dark"
                  ? "rgb(150, 150, 150)"
                  : "rgb(239, 239, 239)",
            }}
            onClick={togglePopupDevice}
          >
            Device:
            <span style={styles.ip}>100.64.0.19</span>
            <OnlineIcon style={styles.onlineIcon} />
          </button>
          {isOpenDevice && (
            <TableContainer component={Paper}>
              <Table sx={{ minWidth: 600 }} aria-label="simple table">
                <TableHead>
                  <TableRow>
                    <TableCell>Hostname</TableCell>
                    <TableCell align="center">IP Address</TableCell>
                    <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                    <TableCell align="center">Peering Method</TableCell>
                    <TableCell align="right">Connection Status</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {deviceRows.map((row, index) => (
                    <TableRow
                      key={row.hostname}
                      sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                    >
                      <TableCell
                        component="th"
                        scope="row"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.hostname}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.ipAddress}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.latency}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.peeringMethod}
                      </TableCell>
                      <TableCell
                        align="right"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.connectionStatus}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </div>
        <div>
          <button
            style={{
              ...styles.relay,
              background:
                theme.palette.mode === "dark"
                  ? "rgb(150, 150, 150)"
                  : "rgb(239, 239, 239)",
            }}
            onClick={togglePopupRelay}
          >
            Relay:
            <span style={styles.ip}>100.64.0.26</span>
            <HighlightOffIcon style={styles.offlineIcon} />
          </button>
          {isOpenRelay && (
            <TableContainer component={Paper}>
              <Table sx={{ minWidth: 600 }} aria-label="simple table">
                <TableHead>
                  <TableRow>
                    <TableCell>Hostname</TableCell>
                    <TableCell align="center">IP Address</TableCell>
                    <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                    <TableCell align="center">Peering Method</TableCell>
                    <TableCell align="right">Connection Status</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {relayRows.map((row, index) => (
                    <TableRow
                      key={row.hostname}
                      sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                    >
                      <TableCell
                        component="th"
                        scope="row"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.hostname}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.ipAddress}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.latency}
                      </TableCell>
                      <TableCell
                        align="center"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.peeringMethod}
                      </TableCell>
                      <TableCell
                        align="right"
                        style={{
                          textDecoration: index === 0 ? "underline" : "none",
                        }}
                      >
                        {row.connectionStatus}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
>>>>>>> d27293ad2dfbb2f43263acd07a3a532e610ff135
        </div>
      </div>
    </div>
  );
};

const styles = {
  device: {
<<<<<<< HEAD
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '9px 16px',
    border: 'none',
    borderTopRightRadius: '20px',
    outline: '2px solid green',
    boxShadow: '0 0 5px 2px rgba(0, 255, 0, 0.5)',
    display: 'flex',
    alignItems: 'center',
  },
  relay: {
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '9px 16px',
    border: 'none',
    borderRadius: '50px',
    outline: '2px solid red',
    boxShadow: '0 0 5px 2px rgba(255, 0, 0, 0.5)',
    display: 'flex',
    alignItems: 'center',
  },
  ip: {
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '12px 16px',
    marginRight: '-15px',
  },
  separator: {
    height: '40px',
    width: '2px',
    background: 'rgba(100, 100, 100, 0.5)',
    marginLeft: '15px',
  },
  deviceIcon: {
    fontSize: '20px',
    marginRight: '8px',
  },
  relayIcon:{
    fontSize: '20px',
    marginRight: '8px',
  },
  onlineIcon: {
    color: 'green',
    fontSize: '20px',
    marginLeft: '15px',
  },
  offlineIcon: {
    color: 'red',
    fontSize: '20px',
    marginLeft: '15px',
=======
    fontSize: "12px",
    fontWeight: "bold",
    padding: "9px 16px",
    //borderColor: 'green',
    //borderWidth: 2,
    border: "none",
    borderTopRightRadius: "20px",
    outline: "2px solid green",
    boxShadow: "0 0 5px 2px rgba(0, 255, 0, 0.5)",
    display: "flex",
    alignItems: "center",
  },
  relay: {
    fontSize: "12px",
    fontWeight: "bold",
    padding: "9px 16px",
    //borderColor: 'red',
    //borderWidth: 2,
    border: "none",
    borderRadius: "50px",
    outline: "2px solid red",
    boxShadow: "0 0 5px 2px rgba(255, 0, 0, 0.5)",
    display: "flex",
    alignItems: "center",
  },
  ip: {
    fontSize: "12px",
    fontWeight: "bold",
    padding: "12px 16px",
    marginLeft: "10px",
  },
  onlineIcon: {
    color: "green",
    fontSize: "20px",
    marginLeft: "10px",
  },
  offlineIcon: {
    color: "red",
    fontSize: "20px",
    marginLeft: "10px",
>>>>>>> d27293ad2dfbb2f43263acd07a3a532e610ff135
  },
};

export default Dashboard;
