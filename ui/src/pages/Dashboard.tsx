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
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import { useTheme } from "@mui/material/styles";

import CardImage from "../wordmark.png";
import CardImageDark from "../wordmark_dark.png";
import { backend } from "../common/Api";

function createData(
  ip: string,
  hostname: string,
  latency: number,
) {
  return { ip, hostname, latency };
}

const deviceRows = [
  createData("100.64.0.19", "80cbb9be04cc", 25),
  createData("100.64.0.26", "48e5326fc084", 64),
  createData("100.64.0.25", "194712efa971", 47),
];

const relayRows = [
  createData("100.64.0.26", "48e5326fc084", 64),
  createData("100.64.0.19", "80cbb9be04cc", 25),
  createData("100.64.0.25", "194712efa971", 47),
]

const Dashboard: React.FC = () => {
  const theme = useTheme();

  const [isOpenDevice, setIsOpenDevice] = useState(false);
  const [isOpenRelay, setIsOpenRelay] = useState(false);
  const togglePopupDevice = () => {
    setIsOpenDevice(!isOpenDevice);
  }
  const togglePopupRelay = () => {
    setIsOpenRelay(!isOpenRelay);
  }
 
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
        <div style={{ display: 'flex', flexDirection: 'row' }}>
          <div style={{ marginRight: '50px'}}>
            <button style={styles.device}
            onClick={togglePopupDevice}>
              Device:
              <span style={styles.ip}>100.64.0.19</span>
              <OnlineIcon style={styles.onlineIcon} />
            </button>
            {isOpenDevice && (
                  <TableContainer component={Paper}>
                  <Table sx={{ minWidth: 400 }} aria-label="simple table">
                    <TableHead>
                      <TableRow>
                        <TableCell>IP Address</TableCell>
                        <TableCell align="center">Hostname</TableCell>
                        <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {deviceRows.map((row, index) => (
                        <TableRow
                          key={row.ip}
                          sx={{ '&:last-child td, &:last-child th': { border: 0 } }}
                        >
                          <TableCell component="th" scope="row" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>
                            {row.ip}
                          </TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.hostname}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.latency}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
          </div>
          <div>
            <button style={styles.relay}
            onClick={togglePopupRelay}>
              Relay:
              <span style={styles.ip}>100.64.0.26</span>
              <HighlightOffIcon style={styles.offlineIcon} />
            </button>
            {isOpenRelay && (
                  <TableContainer component={Paper}>
                  <Table sx={{ minWidth: 400 }} aria-label="simple table">
                    <TableHead>
                      <TableRow>
                        <TableCell>IP Address</TableCell>
                        <TableCell align="center">Hostname</TableCell>
                        <TableCell align="center">Latency&nbsp;(ms)</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {relayRows.map((row, index) => (
                        <TableRow
                          key={row.ip}
                          sx={{ '&:last-child td, &:last-child th': { border: 0 } }}
                        >
                          <TableCell component="th" scope="row" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>
                            {row.ip}
                          </TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.hostname}</TableCell>
                          <TableCell align="center" style={{textDecoration: index === 0 ? 'underline' : 'none'}}>{row.latency}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
            )}
          </div>
        </div>
      </div>
  );
};

const styles = {
  device: {
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '9px 16px',
    backgroundColor: 'green',
    borderColor: 'green',
    borderTopRightRadius: '20px',
    display: 'flex',
    alignItems: 'center',
  },
  relay: {
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '9px 16px',
    backgroundColor: 'red',
    borderColor: 'red',
    borderRadius: '50px',
    display: 'flex',
    alignItems: 'center',
  },
  ip: {
    fontSize: '12px',
    fontWeight: 'bold',
    padding: '12px 16px',
    backgroundColor: 'gray',
    borderColor: 'gray',
    marginLeft: '10px',
  },
  onlineIcon: {
    color: 'white',
    fontSize: '20px',
    marginLeft: '10px',
  },
  offlineIcon: {
    color: 'white',
    fontSize: '20px',
    marginLeft: '10px',
  },
};

export default Dashboard;
