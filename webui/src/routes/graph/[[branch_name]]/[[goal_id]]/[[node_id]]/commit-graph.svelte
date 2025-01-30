<script lang="ts">
	import {
		isBranchTargetLocator,
		isCommitGraphLocator,
		isNodeLocator,
		locatorFromString,
		locatorToJSON,
		type BranchTargetGraphLocators,
		type BranchTargetLocator,
		type CommitGraphLocator,
		type CommitGraphLocators,
		type CommitGraphLocatorsNode,
		type NodeLocator
	} from '$lib';
	import Graph from 'graphology';
	import Sigma from 'sigma';
	import forceAtlas2, { type ForceAtlas2Settings } from 'graphology-layout-forceatlas2';
	import FA2Layout from 'graphology-layout-forceatlas2/worker';
	import circular from 'graphology-layout/circular';
	import { onMount } from 'svelte';
	interface Props {
		graph: CommitGraphLocators;
		selectedNode: NodeLocator | undefined;
		onSelectNode: (locator: NodeLocator) => void;
	}
	const props: Props = $props();

	let renderer: Sigma | undefined;
	let container: HTMLElement;
	let layout: FA2Layout | undefined;

	let settings: ForceAtlas2Settings | undefined;
	const colorForNode = (node: CommitGraphLocatorsNode) => {
		if (node.state === 'node_state_done') {
			if (node.result === 'node_result_success') {
				return '#1B2F33';
			}
			if (node.result === 'node_result_failure') {
				return '#F72C25';
			}
			if (node.result === 'node_result_syntax_failure') {
				return '#2E0014';
			}
			if (node.result === 'node_result_depth_exhaustion') {
				return '#F17105';
			}
			return '#0000ff';
		} else if (node.state === 'node_awaiting_goal_setup') {
			return '#3F7D20';
		} else if (node.state === 'node_state_running_goal_setup') {
			return '#3F7D20';
		} else if (node.state === 'node_awaiting_compilation') {
			return '#F4E0FD';
		} else if (node.state === 'node_state_running_compilation') {
			return '#F6AE2D';
		} else if (node.state === 'node_awaiting_inference') {
			return '#A4B0F5';
		} else if (node.state === 'node_state_running_inference') {
			return '#7D80DA';
		}
	};

	function updateGraph({ graph, selectedNode, onSelectNode }: Props) {
		if (!container) return;

		// Create graph object only if it doesn't exist
		if (!renderer) {
			const graphObject = new Graph();
			layout = new FA2Layout(graphObject);

			renderer = new Sigma(graphObject, container, { minCameraRatio: 0.01, maxCameraRatio: 2 });

			renderer.on('clickNode', (event) => {
				const node = event.node;
				const locator = locatorFromString(node);
				if (isNodeLocator(locator)) {
					onSelectNode(locator);
				}
			});
		}

		const graphObject = renderer.getGraph();

		// Update branch target nodes
		const existingNodes = new Set(graphObject.nodes());

		// Add/update subgraph nodes and edges
		const existingEdges = new Set(graphObject.edges());

		// Add/update branch target nodes
		let numNewNodes = 0;
		let maxDepth = 0;
		for (const node of graph.nodes) {
			maxDepth = Math.max(maxDepth, node.depth);
		}
		for (const node of graph.nodes) {
			const nodeId = locatorToJSON(node.locator);
			const isSelected =
				selectedNode && locatorToJSON(node.locator) === locatorToJSON(selectedNode);
			const objSize =
				((maxDepth - node.depth + 1) * Math.max(15, 1)) / Math.pow(graph.nodes.length, 0.45);
			if (graphObject.hasNode(nodeId)) {
				graphObject.setNodeAttribute(nodeId, 'color', colorForNode(node));
				graphObject.setNodeAttribute(nodeId, 'label', node.depth);
				graphObject.setNodeAttribute(nodeId, 'size', objSize);
			} else {
				numNewNodes++;
				graphObject.addNode(nodeId, {
					x: 0,
					y: 0,
					size: objSize,
					color: colorForNode(node),
					label: node.depth
				});
			}
			existingNodes.delete(nodeId);
		}
		for (const node of graph.nodes) {
			const nodeId = locatorToJSON(node.locator);
			// Handle edges
			for (const childNode of node.children) {
				const childEdgeId = graphObject.edge(nodeId, locatorToJSON(childNode));
				if (!childEdgeId) {
					graphObject.addEdge(nodeId, locatorToJSON(childNode));
				} else {
					existingEdges.delete(childEdgeId);
				}
			}
		}
		if (numNewNodes == graph.nodes.length) {
			circular.assign(graphObject);
		}

		// Remove nodes and edges that no longer exist
		for (const nodeId of existingNodes) {
			graphObject.dropNode(nodeId);
		}
		for (const edgeId of existingEdges) {
			graphObject.dropEdge(edgeId);
		}

		// Start layout if not already started
		//circular.assign(graphObject);
		if (!settings) {
			settings = forceAtlas2.inferSettings(graphObject);
		}
		//forceAtlas2.assign(graphObject, { settings, iterations: numNewNodes / 5 });
		if (!layout?.isRunning()) {
			layout?.start();
		}

		renderer.refresh();
	}

	$effect(() => {
		updateGraph(props);
	});

	onMount(() => {
		container = document.getElementById('commit-graph') as HTMLElement;
		updateGraph(props);

		return () => {
			renderer?.kill();
			layout?.stop();
		};
	});
</script>

<div class="border-1 h-fit w-fit border">
	<div id="commit-graph" style="height: 500px; width: 500px;"></div>
</div>
