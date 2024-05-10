import React from "react";

import { Handle, Position } from "reactflow";
import { RelayNodeComponent, RelayNodeData } from "./RelayNode";

interface CustomRelayNodeProps {
  data: RelayNodeData;
}

//This provides the basic foundation for a ReactFlow relay node
//The actual design of the node is in RelayNode.tsx
const CustomRelayNode: React.FC<CustomRelayNodeProps> = ({ data }) => {
  return (
    <>
      <Handle
        type="target"
        position={Position.Top}
        style={{ left: "50%", top: "50%", zIndex: -1 }}
      />
      <RelayNodeComponent data={data} />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ left: "50%", bottom: "50%", zIndex: -1 }}
      />
    </>
  );
};

export default CustomRelayNode;
