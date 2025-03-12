<script lang="ts">
	import {
		isNodeLocator,
		locatorFromString,
		locatorToJSON,
		type CommitGraphLocators,
		type CommitGraphLocatorsNode,
		type NodeLocator
	} from '$lib';
	import Graph from 'graphology';
	import Sigma from 'sigma';
	import forceAtlas2, { type ForceAtlas2Settings } from 'graphology-layout-forceatlas2';
	import FA2Layout from 'graphology-layout-forceatlas2/worker';
	import ForceLayout from 'graphology-layout-force/worker';
	import circular from 'graphology-layout/circular';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
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
	const colorForNode = (node: CommitGraphLocatorsNode, isSelected: boolean) => {
		if (isSelected) {
			return '#00ff00';
		}
		if (node.state === 'node_state_done') {
			if (node.result === 'node_result_success') {
				return '#30ee30';
			}
			if (node.result === 'node_result_failure') {
				return '#F72C25';
			}
			if (node.result === 'node_result_syntax_failure') {
				return '#2E0014';
			}
			if (node.result === 'node_result_depth_exhaustion') {
				return '#FF31F5';
			}
			if (node.result === 'node_result_terminated') {
				return '#EAF157';
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
	const labelForNode = (node: CommitGraphLocatorsNode) => {
		let label = String(node.depth || 'root');
		if (node.metadata.was_manually_created) {
			label += '-M';
		}
		// prefer label over star emoji
		if (node.metadata.is_favorite && !node.metadata.label) {
			label += '-🌟';
		}
		if (node.metadata.label) {
			label += `-${node.metadata.label}`;
		}
		return label;
	};
	let firstTime = true;

	function updateGraph({ graph, selectedNode, onSelectNode }: Props) {
		if (!container) return;

		// Create graph object only if it doesn't exist
		if (!renderer) {
			const graphObject = new Graph();
			if (graph.nodes.length > 100) {
				layout = new FA2Layout(graphObject, { settings: { gravity: 1.5, adjustSizes: true } });
			} else {
				layout = new ForceLayout(graphObject);
			}

			renderer = new Sigma(graphObject, container, {
				minCameraRatio: 0.01,
				maxCameraRatio: 2,
				renderEdgeLabels: true
			});

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
		let maxDepth = 0;
		for (const node of graph.nodes) {
			maxDepth = Math.max(maxDepth, node.depth);
		}
		for (const node of graph.nodes) {
			const nodeId = locatorToJSON(node.locator);
			const isSelected =
				(selectedNode && locatorToJSON(node.locator) === locatorToJSON(selectedNode)) ?? false;
			const objSize =
				((maxDepth - Math.pow(node.depth, 0.8) + 1) * Math.max(10, 1)) /
				Math.pow(graph.nodes.length, 0.45);
			if (graphObject.hasNode(nodeId)) {
				graphObject.setNodeAttribute(nodeId, 'color', colorForNode(node, isSelected));
				graphObject.setNodeAttribute(nodeId, 'label', labelForNode(node));
				graphObject.setNodeAttribute(nodeId, 'size', objSize);
			} else {
				graphObject.addNode(nodeId, {
					x: Math.random() * 100,
					y: Math.random() * 100,
					size: objSize,
					color: colorForNode(node, isSelected),
					label: labelForNode(node)
				});
			}
			existingNodes.delete(nodeId);
		}
		for (const node of graph.nodes) {
			const nodeId = locatorToJSON(node.locator);
			// Handle edges
			for (const [index, childNode] of node.children.entries()) {
				const childEdgeId = graphObject.edge(nodeId, locatorToJSON(childNode));
				let label = undefined;
				if (node.children_advantages.length > 0) {
					label = String(Math.round(node.children_advantages[index] * 10000) / 10000);
				}
				if (!childEdgeId) {
					graphObject.addEdge(nodeId, locatorToJSON(childNode), {
						label: label
					});
				} else {
					graphObject.setEdgeAttribute(childEdgeId, 'label', label);
					existingEdges.delete(childEdgeId);
				}
			}
		}
		if (firstTime) {
			circular.assign(graphObject);
			firstTime = false;
		}

		try {
			// Remove nodes and edges that no longer exist
			for (const nodeId of existingNodes) {
				graphObject.dropNode(nodeId);
			}
			for (const edgeId of existingEdges) {
				graphObject.dropEdge(edgeId);
			}
		} catch (e) {
			console.error(e);
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
	const quickSelectFavorites = $derived(
		props.graph.nodes
			.filter(
				(node) =>
					node.metadata.is_favorite || node.metadata.was_manually_created || node.metadata.label
			)
			.toSorted(
				(a, b) =>
					labelForNode(a).localeCompare(labelForNode(b)) ||
					b.locator.node_id.localeCompare(a.locator.node_id)
			)
	);
</script>

<div class="flex h-full min-h-0 flex-col">
	<div class="flex-none">
		<div id="commit-graph" style="height: 500px; width: 500px;" class="border"></div>
	</div>
	<!-- node selector for favorites -->
	<h4>Quick Select</h4>
	<div class="flex flex-col gap-2 overflow-y-auto">
		{#each quickSelectFavorites as node}
			{@const is_selected =
				props.selectedNode && locatorToJSON(node.locator) === locatorToJSON(props.selectedNode)}
			<Button
				variant={is_selected ? 'default' : 'outline'}
				onclick={() => props.onSelectNode(node.locator)}
			>
				{labelForNode(node)}
			</Button>
		{/each}
	</div>
</div>
