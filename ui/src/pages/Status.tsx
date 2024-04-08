import ReactFlow, {
  addEdge,
  MiniMap,
  Controls,
  Background,
  ReactFlowProvider,
} from "reactflow";
import {
  ArrayField,
  AutocompleteInput,
  BooleanField,
  BooleanFieldProps,
  BulkDeleteButton,
  BulkExportButton,
  Datagrid,
  DateField,
  Edit,
  List,
  ReferenceField,
  ReferenceInput,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TextField,
  TextInput,
  useGetIdentity,
  useRecordContext,
} from "react-admin";
import "reactflow/dist/style.css";
import React,{ useEffect, useState } from "react";
//Imports our custom nodes
import CustomDeviceNode from "../components/CustomDeviceNode";
import CustomRelayNode from "../components/CustomRelayNode";

//Defines the imported custom nodes
const nodeTypes = {
  customDeviceNode: CustomDeviceNode,
  customRelayNode: CustomRelayNode,
};

// Mock JSON data
const jsonData = [
  {
    "wg_ip": "100.64.0.2",
    "is_reachable": true,
    "hostname": "ip-172-31-26-233.us-east-2.compute.internal",
    "latency": "59.04ms",
    "method": "relay-node-peer"
  },
  {
    "wg_ip": "100.64.0.3",
    "is_reachable": true,
    "hostname": "nuc.lan",
    "latency": "116.65ms",
    "method": "via-relay"
  },
  {
    "wg_ip": "101.64.0.3",
    "is_reachable": false,
    "hostname": "bill.test",
    "latency": "16.65ms",
    "method": "via-relay"
  }
];

// Defines the data structure for a device node in the network graph.
interface DeviceNodeData {
  id: string;
  ip: string;
  hostname: string;
  latency: number;
  peeringMethod: string;
  online: boolean;
}
// Defines the structure for incoming node data. 
interface NodeData {
  wg_ip: string;
  is_reachable: boolean;
  hostname: string;
  latency: string;
  method: string;
}

// Describes the structure of a node within the React Flow graph.
interface Node {
  id: string;
  type: string;
  data: DeviceNodeData;
  position: { x: number; y: number };
}
// Represents the connection or edge between two nodes in the React Flow graph.
interface Edge {
  id: string;
  source: string;
  target: string;
  animated: boolean;
}



// React functional component for rendering the network graph.
export const GraphComponent: React.FC = () => {
// Initialize state for nodes and edges with empty arrays.
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);


  
  useEffect(() => {
    // Transform each item in the mock JSON data into a Node object for React Flow.
    // This includes setting the node's type, data, and random position(for now).
    const generatedNodes: Node[] = jsonData.map((item: NodeData, index: number) => ({
        id: `${index + 1}`,
      type: 'customDeviceNode',
      data: {
        id: `${index + 1}`, 
        ip: item.wg_ip,
        hostname: item.hostname,
        latency: parseFloat(item.latency), 
        peeringMethod: item.method,
        online: item.is_reachable,
    },
      position: { x: Math.random() * window.innerWidth, y: Math.random() * window.innerHeight },//TODO: Have a more organized method of ordering nodes
    }));

    // Generates Edge objects connecting the nodes. Animation indicates reachability.
    const generatedEdges: Edge[] = generatedNodes.map((node, index) => {
      if (index < generatedNodes.length - 1) { // Check to ensure we do not exceed the array bounds.
        return {
          id: `e${node.id}-${generatedNodes[index + 1].id}`,
          source: node.id,
          target: generatedNodes[index + 1].id,
          animated: !jsonData[index].is_reachable,
        };
      }
      return null;
    }).filter((edge): edge is Edge => edge !== null); //Remove nulls from the array

    // Update the state with the generated nodes and edges.
    setNodes(generatedNodes);
    setEdges(generatedEdges);
  }, []);
  //TODO: Impliment the code to run every 5 seconds rather than just once(Code is ready just needs to be tested)


//Draws the graph, passes the custom nodes and edges, and impliments basic ReactFlow controls such as navigation, zooming, etc.
  return (
    <ReactFlowProvider>
      <div style={{ height: '90vh' }}> 
        <ReactFlow  
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}>
          <MiniMap />
          <Controls />
          <Background />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
};


export const StatusList = () => (
  <List>
    <Datagrid>
      <TextField label="ID" source="user_id" />
      <TextField label="Hostname" source="hostname" />
      <TextField label="Wireguard IP" source="wg_ip" />
      <TextField label="IsReachable" source="is_reachable" />
      <TextField label="Latency" source="latency" />
      <TextField label="Method" source="method" />
    </Datagrid>
  </List>
);
