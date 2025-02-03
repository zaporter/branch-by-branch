<script lang="ts">
	import {
		createNodeStatsQuery,
		createRequestNodeTerminationMutation,
		createDeleteNodeMutation,
		type NodeLocator
	} from '$lib';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { useQueryClient } from '@tanstack/svelte-query';

	interface Props {
		locator: NodeLocator;
		unselectNode: () => void;
	}
	const props: Props = $props();
	const query = $derived(createNodeStatsQuery(props.locator));
	const requestNodeTerminationMutation = $derived(
		createRequestNodeTerminationMutation(props.locator)
	);
	const deleteNodeMutation = $derived(createDeleteNodeMutation(props.locator));
	const queryClient = useQueryClient();

	const terminateNode = $derived(async () => {
		const res = await $requestNodeTerminationMutation.mutateAsync();
		console.log(res);
		await $query.refetch();
	});

	const deleteNode = $derived(async () => {
		await $deleteNodeMutation.mutateAsync();
		props.unselectNode();
		// everything
		queryClient.invalidateQueries({ queryKey: [] });
	});
</script>

{#if $query.isLoading}
	<p>Loading...</p>
{:else if $query.isError}
	<p>Error: {$query.error.message}</p>
{:else if $query.data}
	{@const data = $query.data}

	<div class="flex justify-end">
		<AlertDialog.Root>
			<AlertDialog.Trigger class={buttonVariants({ variant: 'outline' })}>
				Show Dialog
			</AlertDialog.Trigger>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Are you absolutely sure?</AlertDialog.Title>
					<AlertDialog.Description>
						This action cannot be undone. This will permanently delete your account and remove your
						data from our servers.
					</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action>Continue</AlertDialog.Action>
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
		<Button variant="destructive" onclick={terminateNode} disabled={data.termination_requested}>
			{data.termination_requested ? 'Termination Requested' : 'Request Termination'}
		</Button>
		<Button variant="destructive" onclick={deleteNode}>🗑️</Button>
	</div>

	<dl class="text-xs [&_dd]:ml-4 [&_dd]:font-normal [&_dt]:font-semibold">
		<dt>Branch Name</dt>
		<dd>{data.branch_name}</dd>
		<dt>Depth</dt>
		<dd>{data.depth}</dd>
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
