import React from "react";
import { Handle, Position, NodeProps } from "reactflow";
import "@mui/material";
import OnlineIcon from "@mui/icons-material/CheckCircleOutline";
import OfflineIcon from "@mui/icons-material/HighlightOff";

interface DeviceNodeData {
  id: string;
  hostname: string;
  ipAddress: string;
  latency: number;
  peeringMethod: string;
  connectionStatus: boolean;
}

interface CustomDeviceNodeProps {
  data: DeviceNodeData;
}

const CustomDeviceNode: React.FC<CustomDeviceNodeProps> = ({ data }) => {
  const { hostname, ipAddress, latency, peeringMethod, connectionStatus } =
    data;

  return (
    <div
      style={{
        border: "1px solid #ddd",
        borderRadius: "4px",
        padding: "10px",
        backgroundColor: "#fff",
      }}
    >
      <div>{hostname}</div>
      <div>{ipAddress}</div>
      <div>{`${latency} ms`}</div>
      <div>{peeringMethod}</div>
      <div>{connectionStatus ? <OnlineIcon /> : <OfflineIcon />}</div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
};

export default CustomDeviceNode;
