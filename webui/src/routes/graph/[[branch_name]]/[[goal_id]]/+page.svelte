<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { createBranchTargetGraphQuery, createPingQuery, type CommitGraphLocator } from '$lib';
	import BranchTargetsGraph from './branch-targets-graph.svelte';

	const pingQuery = createPingQuery();
	const branchTargetGraphQuery = createBranchTargetGraphQuery();

	const onSelectCommitGraph = (locator: CommitGraphLocator) => {
		goto(`/graph/${locator.branch_target_locator.branch_name}/${locator.goal_id}`);
	};

	let selectedCommitGraph: CommitGraphLocator | undefined = $derived.by(() => {
		const branchName = $page.params.branch_name;
		const goalId = $page.params.goal_id;

		if (!branchName || !goalId) {
			return undefined;
		}

		return {
			branch_target_locator: {
				branch_name: branchName
			},
			goal_id: goalId
		};
	});

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
		{selectedCommitGraph}
		{onSelectCommitGraph}
	/>
{/if}
