import React, { FunctionComponent } from "react";
import {
  Card,
  CardHeader,
  CardContent,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Button,
  Typography,
} from "@mui/material";
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import { styled } from "@mui/material/styles";

export interface RelayNodeData {
  id: string;
  ip: string;
  hostname: string;
  latency: number;
  peeringMethod: string;
  online: boolean;
}

interface RelayNodeComponentProps {
  data: RelayNodeData;
}

// Custom styling
const CustomCard = styled(Card)(({ theme }) => ({
  borderRadius: "25px 25px 25px 25px",
  border: "3px solid blue",
  marginBottom: "1rem",
  "& .MuiCardHeader-title": {
    fontSize: "12px",
    fontWeight: "bold",
  },
  "& .MuiCardContent-root": {
    paddingTop: 0,
  },
}));

export const RelayNodeComponent: FunctionComponent<RelayNodeComponentProps> = ({
  data,
}) => {
  const { ip, hostname, latency, peeringMethod, online } = data;
  const [isOpen, setIsOpen] = React.useState(false);

  const toggleOpen = () => setIsOpen(!isOpen);

  return (
    <CustomCard>
      <CardHeader
        title={<Typography variant="h6">Relay: {hostname}</Typography>}
        action={
          <Button onClick={toggleOpen}>
            {online ? (
              <OnlineIcon color="success" />
            ) : (
              <HighlightOffIcon color="error" />
            )}
          </Button>
        }
      />
      {isOpen && (
        <CardContent>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Hostname</TableCell>
                <TableCell>IP Address</TableCell>
                <TableCell>Connection Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              <TableRow>
                <TableCell>{hostname}</TableCell>
                <TableCell>{ip}</TableCell>
                <TableCell>{online ? "Reachable" : "Unreachable"}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      )}
    </CustomCard>
  );
};
