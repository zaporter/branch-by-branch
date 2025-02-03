<script lang="ts">
	import {
		createNodeStatsQuery,
		createRequestNodeTerminationMutation,
		type NodeLocator
	} from '$lib';
	interface Props {
		locator: NodeLocator;
	}
	const props: Props = $props();
	const query = $derived(createNodeStatsQuery(props.locator));
	const requestNodeTerminationMutation = $derived(
		createRequestNodeTerminationMutation(props.locator)
	);

	const terminateNode = $derived(async () => {
		const res = await $requestNodeTerminationMutation.mutateAsync();
		console.log(res);
		await $query.refetch();
	});
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
		<dt>Termination Requested</dt>
		<dd>{data.termination_requested}</dd>
		<dt>Metadata</dt>
		<dd><pre class="whitespace-pre-wrap">{JSON.stringify(data.metadata, null, 2)}</pre></dd>
		<dt>State</dt>
		<dd>{data.state}</dd>
		<dt>Result</dt>
		<dd>{data.result}</dd>
		<dt>Inference Output (from parent)</dt>
		<dd><pre class="whitespace-pre-wrap">{data.inference_output}</pre></dd>
		<dt>Action Outputs (from applying parsed(inference output) to the parent branch)</dt>
		<dd>
			{#each data.action_outputs ?? [] as action_output}
				<pre class="whitespace-pre-wrap">{action_output.action_name}: {action_output.text}</pre>
			{/each}
		</dd>
		<dt>Compilation Result (from applying parsed(inference output) to the parent branch)</dt>
		<dd>
			{#if data.compilation_result}
				<pre class="whitespace-pre-wrap">{data.compilation_result.out}</pre>
			{/if}
		</dd>
		<dt>Prompt (that is used to create children)</dt>
		<dd><pre class="whitespace-pre-wrap">{data.prompt}</pre></dd>
	</dl>
{/if}
