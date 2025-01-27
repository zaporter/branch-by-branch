<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		branchLocatorFromUnknown,
		commitGraphLocatorFromUnknown,
		createBranchTargetGraphQuery,
		createCommitGraphQuery,
		createPingQuery,
		nodeLocatorFromUnknown,
		type BranchTargetLocator,
		type CommitGraphLocator,
		type NodeLocator,
		type UnknownLocator
	} from '$lib';
	import BranchTargetsGraph from './branch-targets-graph.svelte';
	import CommitGraph from './commit-graph.svelte';

	const pingQuery = createPingQuery();
	const branchTargetGraphQuery = createBranchTargetGraphQuery();

	let currentLocator: UnknownLocator | undefined = $derived.by(() => {
		const branchName = page.params.branch_name;
		const goalId = page.params.goal_id;
		const nodeId = page.params.node_id;

		if (!branchName) {
			return undefined;
		}
		const branchLocator: BranchTargetLocator = {
			branch_name: branchName
		};

		const commitGraphLocator: CommitGraphLocator | undefined = goalId
			? {
					branch_target_locator: branchLocator,
					goal_id: goalId
				}
			: undefined;

		const nodeLocator: NodeLocator | undefined =
			nodeId && commitGraphLocator
				? {
						commit_graph_locator: commitGraphLocator,
						node_id: nodeId
					}
				: undefined;

		return nodeLocator ?? commitGraphLocator ?? branchLocator;
	});

	let currentBranchTargetLocator = $derived(branchLocatorFromUnknown(currentLocator));
	let currentCommitGraphLocator = $derived(commitGraphLocatorFromUnknown(currentLocator));
	let currentNodeLocator = $derived(nodeLocatorFromUnknown(currentLocator));

	const commitGraphQuery = $derived(
		currentCommitGraphLocator ? createCommitGraphQuery(currentCommitGraphLocator) : undefined
	);
	const onSelectBranchTarget = (locator: BranchTargetLocator) => {
		goto(`/graph/${locator.branch_name}`);
	};
	const onSelectCommitGraph = (locator: CommitGraphLocator) => {
		goto(`/graph/${locator.branch_target_locator.branch_name}/${locator.goal_id}`);
	};
	const onSelectNode = (locator: NodeLocator) => {
		goto(
			`/graph/${locator.commit_graph_locator.branch_target_locator.branch_name}/${locator.commit_graph_locator.goal_id}/${locator.node_id}`
		);
	};
</script>

{#if $pingQuery.isLoading}
	<p>Loading...</p>
{:else if $pingQuery.isError}
	<p>Error: {$pingQuery.error.message}</p>
{:else}
	<p>{$pingQuery.data}</p>
{/if}

{#if $branchTargetGraphQuery.isLoading}
	<p>Loading...</p>
{:else if $branchTargetGraphQuery.isError}
	<p>Error: {$branchTargetGraphQuery.error.message}</p>
{:else if $branchTargetGraphQuery.data}
	<BranchTargetsGraph
		graph={$branchTargetGraphQuery.data}
		selectedCommitGraph={currentCommitGraphLocator}
		selectedBranchTarget={currentBranchTargetLocator}
		{onSelectCommitGraph}
		{onSelectBranchTarget}
	/>
{/if}

{#if $commitGraphQuery}
	{#if $commitGraphQuery.isLoading}
		<p>Loading...</p>
	{:else if $commitGraphQuery.isError}
		<p>Error: {$commitGraphQuery.error.message}</p>
	{:else if $commitGraphQuery.data}
		<CommitGraph graph={$commitGraphQuery.data} selectedNode={currentNodeLocator} {onSelectNode} />
	{/if}
{/if}
