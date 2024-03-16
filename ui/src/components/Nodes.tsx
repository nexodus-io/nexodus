/*
import React, { useState, FunctionComponent } from 'react';
import * as d3 from 'd3';
import { Card, CardHeader, CardContent, Table, TableBody, TableCell, TableHead, TableRow, Button } from '@mui/material';
import OnlineIcon from '@mui/icons-material/CheckCircleOutline';
import HighlightOffIcon from '@mui/icons-material/HighlightOff';

interface DeviceNode extends d3.SimulationNodeDatum{
  id: string;
  ip: string;
  hostname: string;
  latency: number;
  online: boolean;
}

interface DeviceNodeComponentProps {
  deviceNode: DeviceNode;
}

export const DeviceNodeComponent: FunctionComponent<DeviceNodeComponentProps> = ({ deviceNode }) => {
  const [isOpen, setIsOpen] = useState(false);

  const toggleOpen = () => setIsOpen(!isOpen);

  return (
    <Card variant="outlined" style={{ marginBottom: '1rem' }}>
      <CardHeader
        title={`Device: ${deviceNode.hostname}`}
        action={
          <Button onClick={toggleOpen}>
            {deviceNode.online ? <OnlineIcon color="success" /> : <HighlightOffIcon color="error" />}
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
                <TableCell>{deviceNode.ip}</TableCell>
                <TableCell>{deviceNode.hostname}</TableCell>
                <TableCell>{deviceNode.latency}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      )}
    </Card>
  );
};
*/

