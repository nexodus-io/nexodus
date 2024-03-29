import ReactFlow, { addEdge, MiniMap, Controls, Background, ReactFlowProvider } from 'reactflow';
import 'reactflow/dist/style.css';
import React from 'react';



import CustomDeviceNode from '../components/CustomDeviceNode';
import CustomRelayNode from '../components/CustomRelayNode';

const nodeTypes = { customDeviceNode: CustomDeviceNode, customRelayNode: CustomRelayNode};

const initialNodes = [
  {
    id: '1',
    type: 'customDeviceNode',
    position: { x: 350, y: 0 },
    data: {
      id: '1',
      hostname: 'Device 1',
      ip: '192.168.1.1',
      latency: 47.2,
      peeringMethod: 'Direct',
      online: true,
    },
  },
  {
    id: '2',
    type: 'customRelayNode',
    position: { x: -200, y: 0 },
    data: {
      id: '2',
      hostname: 'Traffic Relay',
      ip: '192.168.1.2',
      latency: 58.0,
      peeringMethod: 'via-relay',
      online: true,
    },
  },
  {
    id: '3',
    type: 'customDeviceNode',
    position: { x: -700, y: 400 },
    data: {
      id: '3',
      hostname: 'Device 2',
      ip: '192.168.1.3',
      latency: 88.5,
      peeringMethod: 'Direct',
      online: true,
    },
  },
  {
    id: '4',
    type: 'customDeviceNode',
    position: { x: -1200, y: 0 },
    data: {
      id: '4',
      hostname: 'Device 3',
      ip: '192.168.1.4',
      latency: 27.1,
      peeringMethod: 'via-node-relay',
      online: true,
    },
  },
  {
    id: '5',
    type: 'customDeviceNode',
    position: { x: -700, y: -400 },
    data: {
      id: '5',
      hostname: 'Device 4',
      ip: '192.168.1.5',
      latency: 38.5,
      peeringMethod: 'Direct',
      online: false,
    },
  }
];

const initialEdges = [
{ id: 'e1-2', source: '1', target: '2', animated: false },
{ id: 'e2-3', source: '2', target: '3', animated: false },
{ id: 'e2-4', source: '2', target: '4', animated: false },
{ id: 'e2-5', source: '2', target: '5', animated: false },
{ id: 'e4-3', source: '4', target: '3', animated: false },
{ id: 'e4-5', source: '4', target: '5', animated: false },
{ id: 'e5-3', source: '5', target: '3', animated: false },
];

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


