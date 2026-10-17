// Types for the network graph visualization
import type { components, operations } from './generated/api';
import type { WithoutNull } from './utils';

type ApiNode = components['schemas']['GraphNodeResponse'];
type ApiEdge = components['schemas']['GraphEdgeResponse'];

// The UI adds circle nodes and edges of its own, and force-graph adds positions while rendering
export interface GraphNode extends Omit<ApiNode, 'type'> {
  type: ApiNode['type'] | 'circle';
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
}

export interface GraphEdge extends Omit<ApiEdge, 'type' | 'source' | 'target'> {
  type: ApiEdge['type'] | 'circle';
  source: string | GraphNode; // Can be ID string or resolved node object
  target: string | GraphNode;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export type GraphResponse = GraphData &
  WithoutNull<operations['network-graph']['responses']['200']['content']['application/json']>;
