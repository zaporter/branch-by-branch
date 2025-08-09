<script lang="ts">
	import {
		createCommitGraphStatsQuery,
		createSetCommitGraphStateMutation,
		type CommitGraphLocator,
		type GraphState
	} from '$lib';
	import { Button } from '$lib/components/ui/button';
	interface Props {
		locator: CommitGraphLocator;
	}
	const props: Props = $props();
	const query = $derived(createCommitGraphStatsQuery(props.locator));
	const setCommitGraphStateMutation = $derived(createSetCommitGraphStateMutation());
	const setCommitGraphState = $derived(async (state: GraphState) => {
		await $setCommitGraphStateMutation.mutateAsync({
			commit_graph_locator: props.locator,
			state: state
		});
		await $query.refetch();
	});
</script>

{#if $query.isLoading}
	<p>Loading...</p>
{:else if $query.isError}
	<p>Error: {$query.error.message}</p>
{:else if $query.data}
	{@const data = $query.data}
	<div class="flex flex-row gap-2">
		<Button onclick={() => setCommitGraphState('graph_in_progress')}>Set to in progress</Button>
		<Button onclick={() => setCommitGraphState('graph_success')}>Set to success</Button>
		<Button onclick={() => setCommitGraphState('graph_failed')}>Set to failed</Button>
	</div>
	<dl class="text-xs [&_dd]:ml-4 [&_dd]:font-normal [&_dt]:font-semibold">
		<dt>State</dt>
		<dd>{data.state}</dd>
		<dt>Goal ID</dt>
		<dd>{data.goal_id}</dd>
	</dl>
{/if}
