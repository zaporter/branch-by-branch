<script lang="ts">
	import { createBranchTargetStatsQuery, type BranchTargetLocator } from '$lib';
	interface Props {
		locator: BranchTargetLocator;
	}
	const props: Props = $props();
	const query = $derived(createBranchTargetStatsQuery(props.locator));
</script>

{#if $query.isLoading}
	<p>Loading...</p>
{:else if $query.isError}
	<p>Error: {$query.error.message}</p>
{:else if $query.data}
	{@const data = $query.data}
	<dl class="text-xs [&_dd]:ml-4 [&_dd]:font-normal [&_dt]:font-semibold">
		<dt>Branch Name</dt>
		<dd>{data.branch_name}</dd>
		<dt>Parent Branch Name</dt>
		<dd>{data.parent_branch_name}</dd>
		<dt>Num Subgraphs</dt>
		<dd>{data.num_subgraphs}</dd>
	</dl>
{/if}
