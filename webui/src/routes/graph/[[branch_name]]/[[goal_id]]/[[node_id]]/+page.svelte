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
	import BranchTargetStats from './branch-target-stats.svelte';
	import BranchTargetsGraph from './branch-targets-graph.svelte';
	import CommitGraphStats from './commit-graph-stats.svelte';
	import CommitGraph from './commit-graph.svelte';
	import NodeStats from './node-stats.svelte';
	import StatsContainer from './stats-container.svelte';

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
	const unselectNode = () => {
		goto(
			`/graph/${currentNodeLocator?.commit_graph_locator.branch_target_locator.branch_name}/${currentNodeLocator?.commit_graph_locator.goal_id}`
		);
	};
</script>

<div class="flex h-full flex-row gap-4 p-4">
	<div class="flex h-full flex-col gap-4">
		{#if $pingQuery.isLoading}
			<p>Loading...</p>
		{:else if $pingQuery.isError}
			<p>Error: {$pingQuery.error.message}</p>
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
				<CommitGraph
					graph={$commitGraphQuery.data}
					selectedNode={currentNodeLocator}
					{onSelectNode}
				/>
			{/if}
		{/if}
	</div>

	<div class="flex h-full h-full w-full flex-col gap-4 overflow-y-auto">
		{#if currentBranchTargetLocator}
			<StatsContainer title={`Branch ${currentBranchTargetLocator.branch_name}`}>
				<BranchTargetStats locator={currentBranchTargetLocator} />
			</StatsContainer>
		{/if}
		{#if currentCommitGraphLocator}
			<StatsContainer title={`Commit Graph ${currentCommitGraphLocator.goal_id}`}>
				<CommitGraphStats locator={currentCommitGraphLocator} />
			</StatsContainer>
		{/if}
		{#if currentNodeLocator}
			<StatsContainer title={`Node ${currentNodeLocator.node_id}`}>
				<NodeStats locator={currentNodeLocator} {unselectNode} selectNode={onSelectNode} />
			</StatsContainer>
		{/if}
	</div>
</div>
