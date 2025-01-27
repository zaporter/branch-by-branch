<script lang="ts">
	import { createCommitGraphStatsQuery, type CommitGraphLocator } from '$lib';
	interface Props {
		locator: CommitGraphLocator;
	}
	const props: Props = $props();
	const query = $derived(createCommitGraphStatsQuery(props.locator));
</script>

{#if $query.isLoading}
	<p>Loading...</p>
{:else if $query.isError}
	<p>Error: {$query.error.message}</p>
{:else if $query.data}
	{@const data = $query.data}
	<dl class="text-xs [&_dd]:ml-4 [&_dd]:font-normal [&_dt]:font-semibold">
		<dt>State</dt>
		<dd>{data.state}</dd>
		<dt>Goal ID</dt>
		<dd>{data.goal_id}</dd>
	</dl>
{/if}
