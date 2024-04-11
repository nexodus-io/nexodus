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
  NotificationType,
  ReferenceField,
  ReferenceInput,
  Show,
  SimpleForm,
  SimpleShowLayout,
  TextField,
  TextInput,
  useGetIdentity,
  useRecordContext,
  useNotify,
} from "react-admin";
import "reactflow/dist/style.css";
import React, { useEffect, useState } from "react";
//Imports our custom nodes
import CustomDeviceNode from "../components/CustomDeviceNode";
import CustomRelayNode from "../components/CustomRelayNode";
import { backend, fetchJson as apiFetchJson } from "../common/Api";

//Defines the imported custom nodes
const nodeTypes = {
  customDeviceNode: CustomDeviceNode,
  customRelayNode: CustomRelayNode,
};

// Mock JSON data
/*const jsonData = [
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
];*/

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
const fetchStatus = async () => {
  const statusData = await apiFetchJson(`${backend}/api/status`, {
    method: "GET",
  });

  return statusData;
};

const GraphComponent = () => {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);

  useEffect(() => {
    const displayStatuses = async () => {
      try {
        const status = await fetchStatus();

        const generatedNodes: Node[] = status.map(
          (item: NodeData, index: number) => ({
            id: `${index + 1}`,
            type: "customDeviceNode",
            data: {
              id: `${index + 1}`,
              ip: item.wg_ip,
              hostname: item.hostname,
              latency: parseFloat(item.latency),
              peeringMethod: item.method,
              online: item.is_reachable,
            },
            position: {
              x: Math.random() * window.innerWidth,
              y: Math.random() * window.innerHeight,
            },
          })
        );

        // Ensure there is at least one node to connect to
        if (generatedNodes.length > 1) {
          const generatedEdges: Edge[] = generatedNodes.slice(1).map((node) => ({
            id: `e${node.id}-1`, // Connects each node to the first node, whose ID is '1'
            source: node.id,
            target: '1',  // This assumes the first node has an ID of '1'
            animated: node.data.online,
          }));

          setNodes(generatedNodes);
          setEdges(generatedEdges);
        }
      } catch (error) {
        console.error("Error fetching or processing data:", error);
      }
    };

    displayStatuses(); // Initial call
  }, []);

  return (
    <ReactFlowProvider>
      <div style={{ height: "90vh" }}>
        <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes}>
          <MiniMap />
          <Controls />
          <Background />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
};

export default GraphComponent;
