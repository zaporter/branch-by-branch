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
		type NodeLocator
	} from '$lib';
	import Graph from 'graphology';
	import Sigma from 'sigma';
	import ForceSupervisor from 'graphology-layout-force/worker';
	import { onMount } from 'svelte';
	interface Props {
		graph: CommitGraphLocators;
		selectedNode: NodeLocator | undefined;
		onSelectNode: (locator: NodeLocator) => void;
	}
	const props: Props = $props();

	let renderer: Sigma | undefined;
	let layout: ForceSupervisor | undefined;
	let container: HTMLElement;

	function updateGraph({ graph, selectedNode, onSelectNode }: Props) {
		if (!container) return;

		const RED = '#ff0000';
		const BLUE = '#0000ff';
		const GREEN = '#00ff00';

		// Create graph object only if it doesn't exist
		if (!renderer) {
			const graphObject = new Graph();
			layout = new ForceSupervisor(graphObject, { isNodeFixed: (_, attr) => attr.highlighted });
			renderer = new Sigma(graphObject, container, { minCameraRatio: 0.5, maxCameraRatio: 2 });

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
		for (const node of graph.nodes) {
			const nodeId = locatorToJSON(node.locator);
			const isSelected =
				selectedNode && locatorToJSON(node.locator) === locatorToJSON(selectedNode);
			if (graphObject.hasNode(nodeId)) {
				graphObject.setNodeAttribute(nodeId, 'color', isSelected ? GREEN : RED);
			} else {
				graphObject.addNode(nodeId, {
					x: graphObject.order ? Math.random() * 100 : 0,
					y: node.depth * 10,
					size: 10,
					color: isSelected ? GREEN : RED
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

		// Remove nodes and edges that no longer exist
		for (const nodeId of existingNodes) {
			graphObject.dropNode(nodeId);
		}
		for (const edgeId of existingEdges) {
			graphObject.dropEdge(edgeId);
		}

		// Start layout if not already started
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
