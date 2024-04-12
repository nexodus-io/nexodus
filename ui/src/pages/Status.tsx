import ReactFlow, {
  addEdge,
  MiniMap,
  Controls,
  Background,
  ReactFlowProvider,
  applyNodeChanges,
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
import React, { useEffect, useState, useCallback } from "react";
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
  },
  {
    "wg_ip": "101.64.0.3",
    "is_reachable": true,
    "hostname": "bill.test2",
    "latency": "16.65ms",
    "method": "direct"
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
        const status = await fetchStatus(); // Fetch the status data

        const generatedNodes: Node[] = [];
        const generatedEdges: Edge[] = [];

        jsonData.forEach((item: NodeData, index: number) => {
          const nodeId = `${index + 1}`;
          const nodeType = item.method === "relay-node-peer" || item.method === "derp-relay" ? "customRelayNode" : "customDeviceNode";

          const newNode: Node = {
            id: nodeId,
            type: nodeType,
            data: {
              id: nodeId,
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
          };

          generatedNodes.push(newNode);

          // Ensure each node connects back to the first node by default if it's not a relay connection
          if (item.method !== "via-relay" && generatedNodes.length > 1) {
            const targetNode = generatedNodes[0]; // Connects back to the first node
            generatedEdges.push({
              id: `e${nodeId}-${targetNode.id}`,
              source: nodeId,
              target: targetNode.id,
              animated: item.is_reachable,
            });
          }

          // Special handling for nodes that should connect via relay
          if (nodeType === "customRelayNode" || item.method === "via-relay") {
            // Find the first custom relay node or use the current one if it's a relay
            const relayNode = generatedNodes.find(n => n.type === "customRelayNode") || newNode;
            if (relayNode && relayNode.id !== nodeId) { // Avoid self-connection for relay nodes
              generatedEdges.push({
                id: `e${nodeId}-${relayNode.id}`,
                source: nodeId,
                target: relayNode.id,
                animated: item.is_reachable,
              });
            }
          }
        });

        setNodes(generatedNodes);
        setEdges(generatedEdges);
      } catch (error) {
        console.error("Error fetching or processing data:", error);
      }
    };

    displayStatuses(); // Initial call
  }, []);

  const onNodeDragStop = (event: any, node: { id: string; position: { x: any; y: any; }; }) => {
    setNodes((currNodes) => currNodes.map((n) => {
      if (n.id === node.id) {
        return {
          ...n,
          position: {
            x: node.position.x,
            y: node.position.y,
          }
        };
      }
      return n;
    }));
  };

  return (
    <ReactFlowProvider>
      <div style={{ height: "90vh" }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodeDragStop={onNodeDragStop}
          nodeTypes={nodeTypes}
        >
          <MiniMap />
          <Controls />
          <Background />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
};

export default GraphComponent;