import React from "react";

import { Handle, Position } from "reactflow";
import { DeviceNodeComponent, DeviceNodeData } from "./Nodes";

interface CustomDeviceNodeProps {
  data: DeviceNodeData;
}

const CustomDeviceNode: React.FC<CustomDeviceNodeProps> = ({ data }) => {
  return (
    <>
      <Handle
        type="target"
        position={Position.Top}
        style={{ left: "50%", top: "50%", zIndex: -1 }}
      />
      <DeviceNodeComponent data={data} />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ left: "50%", bottom: "50%", zIndex: -1 }}
      />
    </>
  );
};

export default CustomDeviceNode;
