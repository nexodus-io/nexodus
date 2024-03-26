// GraphComponent.tsx
import ReactFlow, { addEdge, MiniMap, Controls, Background, ReactFlowProvider } from 'reactflow';
import 'reactflow/dist/style.css';

import React from 'react';



import CustomDeviceNode from '../components/CustomDeviceNode';

const nodeTypes = { customDeviceNode: CustomDeviceNode };

const initialNodes = [
  {
    id: '1',
    type: 'customDeviceNode',
    position: { x: 250, y: 5 },
    data: {
      id: '1',
      hostname: 'Device 1',
      ipAddress: '192.168.1.1',
      latency: 100,
      peeringMethod: 'Direct',
      connectionStatus: true,
    },
  },
  {
    id: '2',
    type: 'customDeviceNode',
    position: { x: 100, y: 150 },
    data: {
      id: '2',
      hostname: 'Device 2',
      ipAddress: '192.168.1.2',
      latency: 200,
      peeringMethod: 'Relay',
      connectionStatus: false,
    },
  },
];

const initialEdges = [{ id: 'e1-2', source: '1', target: '2', animated: true }];

const GraphComponent = () => (
  <ReactFlowProvider>
    <div style={{ height: 500 }}>
      <ReactFlow nodes={initialNodes} edges={initialEdges} nodeTypes={nodeTypes}>
        <MiniMap />
        <Controls />
        <Background />
      </ReactFlow>
    </div>
  </ReactFlowProvider>
);

export default GraphComponent;


