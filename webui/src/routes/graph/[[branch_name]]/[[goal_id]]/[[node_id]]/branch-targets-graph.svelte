<script lang="ts">
	import {
		isBranchTargetLocator,
		isCommitGraphLocator,
		locatorFromString,
		locatorToJSON,
		type BranchTargetGraphLocators,
		type BranchTargetLocator,
		type CommitGraphLocator
	} from '$lib';
	import Graph from 'graphology';
	import Sigma from 'sigma';
	import ForceSupervisor from 'graphology-layout-force/worker';
	import { onMount } from 'svelte';
	interface Props {
		graph: BranchTargetGraphLocators;
		selectedCommitGraph: CommitGraphLocator | undefined;
		selectedBranchTarget: BranchTargetLocator | undefined;
		onSelectCommitGraph: (locator: CommitGraphLocator) => void;
		onSelectBranchTarget: (locator: BranchTargetLocator) => void;
	}
	const props: Props = $props();

	let renderer: Sigma | undefined;
	let layout: ForceSupervisor | undefined;
	let container: HTMLElement;

	function updateGraph({
		graph,
		selectedCommitGraph,
		selectedBranchTarget,
		onSelectCommitGraph,
		onSelectBranchTarget
	}: Props) {
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
				if (isCommitGraphLocator(locator)) {
					onSelectCommitGraph(locator);
				} else if (isBranchTargetLocator(locator)) {
					onSelectBranchTarget(locator);
				}
			});
		}

		const graphObject = renderer.getGraph();

		// Update branch target nodes
		const existingNodes = new Set(graphObject.nodes());

		// Add/update branch target nodes
		for (const branchTarget of graph.branch_targets) {
			const nodeId = locatorToJSON(branchTarget);
			const isSelected =
				selectedBranchTarget && locatorToJSON(branchTarget) === locatorToJSON(selectedBranchTarget);
			if (graphObject.hasNode(nodeId)) {
				graphObject.setNodeAttribute(nodeId, 'label', branchTarget.branch_name);
				graphObject.setNodeAttribute(nodeId, 'color', isSelected ? GREEN : RED);
			} else {
				graphObject.addNode(nodeId, {
					label: branchTarget.branch_name,
					x: graphObject.order ? Math.random() * 100 : 0,
					y: graphObject.order ? Math.random() * 100 : 0,
					size: 20,
					color: isSelected ? GREEN : RED
				});
			}
			existingNodes.delete(nodeId);
		}

		// Add/update subgraph nodes and edges
		const existingEdges = new Set(graphObject.edges());

		for (const subgraph of graph.subgraphs) {
			const nodeId = locatorToJSON(subgraph.commit_graph);
			const isSelected =
				selectedCommitGraph &&
				locatorToJSON(subgraph.commit_graph) === locatorToJSON(selectedCommitGraph);

			if (graphObject.hasNode(nodeId)) {
				graphObject.setNodeAttribute(nodeId, 'label', subgraph.goal_name ?? '');
				graphObject.setNodeAttribute(nodeId, 'color', isSelected ? GREEN : BLUE);
			} else {
				graphObject.addNode(nodeId, {
					label: subgraph.goal_name ?? '',
					x: graphObject.order ? Math.random() * 100 : 0,
					y: graphObject.order ? Math.random() * 100 : 0,
					size: 10,
					color: isSelected ? GREEN : BLUE
				});
			}
			existingNodes.delete(nodeId);

			// Handle edges
			const parentEdgeId = graphObject.edge(locatorToJSON(subgraph.parent_branch_target), nodeId);
			if (!parentEdgeId) {
				graphObject.addEdge(locatorToJSON(subgraph.parent_branch_target), nodeId);
			} else {
				existingEdges.delete(parentEdgeId);
			}

			for (const childBranchTarget of subgraph.children_branch_targets ?? []) {
				const childEdgeId = graphObject.edge(nodeId, locatorToJSON(childBranchTarget));
				if (!childEdgeId) {
					graphObject.addEdge(nodeId, locatorToJSON(childBranchTarget));
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
		container = document.getElementById('branch-targets-graph') as HTMLElement;
		updateGraph(props);

		return () => {
			renderer?.kill();
			layout?.stop();
		};
	});
</script>

<div class="border-1 h-fit w-fit border">
	<div id="branch-targets-graph" style="height: 500px; width: 500px;"></div>
</div>
