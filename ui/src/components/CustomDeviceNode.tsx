import React from 'react';

import { Handle, Position } from 'reactflow';
import { DeviceNodeComponent, DeviceNodeData } from './Nodes';

interface CustomDeviceNodeProps {
  data: DeviceNodeData;
}

const CustomDeviceNode: React.FC<CustomDeviceNodeProps> = ({ data }) => {
  return (
    <>
      <Handle type="target" position={Position.Top} />
      <DeviceNodeComponent data={data} />
      <Handle type="source" position={Position.Bottom} />
    </>
  );
};

export default CustomDeviceNode;



