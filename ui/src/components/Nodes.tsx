
import React, { FunctionComponent } from 'react';
import { Card, CardHeader, CardContent, Table, TableBody, TableCell, TableHead, TableRow, Button } from '@mui/material';
import OnlineIcon from '@mui/icons-material/CheckCircleOutline';
import HighlightOffIcon from '@mui/icons-material/HighlightOff';

interface DeviceNodeData {
  id: string;
  ip: string;
  hostname: string;
  latency: number;
  online: boolean;
}

interface DeviceNodeComponentProps {
  data: DeviceNodeData; 
}

export const DeviceNodeComponent: React.FC<DeviceNodeComponentProps> = ({ data }) => {
  const { ip, hostname, latency, online } = data;
  const [isOpen, setIsOpen] = React.useState(false);

  const toggleOpen = () => setIsOpen(!isOpen);

return (
  <Card variant="outlined" style={{ marginBottom: '1rem' }}>
    <CardHeader
      title={`Device: ${hostname}`}
      action={
        <Button onClick={toggleOpen}>
          {online ? <OnlineIcon color="success" /> : <HighlightOffIcon color="error" />}
        </Button>
      }
    />
    {isOpen && (
      <CardContent>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>IP Address</TableCell>
              <TableCell>Hostname</TableCell>
              <TableCell>Latency (ms)</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            <TableRow>
              <TableCell>{ip}</TableCell>
              <TableCell>{hostname}</TableCell>
              <TableCell>{latency}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    )}
  </Card>
);
};

export type { DeviceNodeData };

