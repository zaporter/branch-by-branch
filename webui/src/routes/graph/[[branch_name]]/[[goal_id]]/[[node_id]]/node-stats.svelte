<script lang="ts">
	import { createNodeStatsQuery, type NodeLocator } from '$lib';
	interface Props {
		locator: NodeLocator;
	}
	const props: Props = $props();
	const query = $derived(createNodeStatsQuery(props.locator));
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
		<dt>Depth</dt>
		<dd>{data.depth}</dd>
		<dt>State</dt>
		<dd>{data.state}</dd>
		<dt>Result</dt>
		<dd>{data.result}</dd>
	</dl>
{/if}
