<script lang="ts">
	import {
		createNodeStatsQuery,
		createRequestNodeTerminationMutation,
		createDeleteNodeMutation,
		createSaveGoldenSampleMutation,
		type NodeLocator,
		createCreateNodeMutation,
		createSetNodeMetadataMutation
	} from '$lib';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { useQueryClient } from '@tanstack/svelte-query';
	import CreateNodeDialog from './create-node-dialog.svelte';
	import { Input } from '$lib/components/ui/input';

	interface Props {
		locator: NodeLocator;
		unselectNode: () => void;
		selectNode: (locator: NodeLocator) => void;
	}
	const props: Props = $props();
	const query = $derived(createNodeStatsQuery(props.locator));
	const requestNodeTerminationMutation = $derived(
		createRequestNodeTerminationMutation(props.locator)
	);
	const deleteNodeMutation = $derived(createDeleteNodeMutation(props.locator));
	const createNodeMutation = $derived(createCreateNodeMutation());
	const setNodeMetadataMutation = $derived(createSetNodeMetadataMutation());
	const saveGoldenSampleMutation = $derived(createSaveGoldenSampleMutation());
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
	const createNode = $derived(async (text: string) => {
		const res = await $createNodeMutation.mutateAsync({
			parent_node_locator: props.locator,
			inference_output: text
		});
		props.selectNode(res.node_locator);
	});
	let saveGoldenSampleDialogOpen = $state(false);
	const saveGoldenSample = $derived(async () => {
		await $saveGoldenSampleMutation.mutateAsync({
			node_locator: props.locator
		});
		saveGoldenSampleDialogOpen = false;
	});
	const toggleFavorite = $derived(async () => {
		if (!$query.data) return;
		await $setNodeMetadataMutation.mutateAsync({
			node_locator: props.locator,
			metadata: { ...$query.data.metadata, is_favorite: !Boolean($query.data.metadata.is_favorite) }
		});
		await $query.refetch();
	});
	let savedLabelText = $derived($query.data?.metadata.label ?? '');
	let labelText = $state('');
	$effect(() => {
		labelText = savedLabelText;
	});
	const setLabel = $derived(async (label: string) => {
		if (!$query.data) return;
		await $setNodeMetadataMutation.mutateAsync({
			node_locator: props.locator,
			metadata: { ...$query.data.metadata, label }
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
	<div class="grid grid-cols-2 gap-2">
		<Button onclick={toggleFavorite}>
			{data.metadata.is_favorite ? '⭐️' : '☆'}
		</Button>
		<Input type="text" bind:value={labelText} onfocusout={() => setLabel(labelText)} />
		<CreateNodeDialog
			parentNodeLocator={props.locator}
			prompt={data.prompt ?? 'no prompt'}
			onCreateNode={createNode}
		/>
		<AlertDialog.Root
			open={saveGoldenSampleDialogOpen}
			onOpenChange={(open) => {
				saveGoldenSampleDialogOpen = open;
			}}
		>
			<AlertDialog.Trigger class={buttonVariants({ variant: 'outline' })}>
				Save Golden Sample
			</AlertDialog.Trigger>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Save Golden Sample</AlertDialog.Title>
					<AlertDialog.Description>Save this node as a golden sample.</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action onclick={saveGoldenSample}>Do it</AlertDialog.Action>
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
		<AlertDialog.Root>
			<AlertDialog.Trigger class={buttonVariants({ variant: 'destructive' })}>
				Delete Node
			</AlertDialog.Trigger>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Are you absolutely sure?</AlertDialog.Title>
					<AlertDialog.Description>
						This is a dangerous operation that can crash the orchestrator & corrupt your graph.
						Please read the source of orchestrator-web.go to understand the implications &
						implementation of this action. At least make a backup of the graph before doing this.
					</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action onclick={deleteNode}>Do it</AlertDialog.Action>
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
		<Button variant="destructive" onclick={terminateNode} disabled={data.termination_requested}>
			{data.termination_requested ? 'Termination Requested' : 'Request Termination'}
		</Button>
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
