import React from "react";

import { Handle, Position } from "reactflow";
import { DeviceNodeComponent, DeviceNodeData } from "./DeviceNode";

interface CustomDeviceNodeProps {
  data: DeviceNodeData;
}

//This provides the basic foundation for a ReactFlow device node
//The actual design of the node is in DeviceNode.tsx
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
