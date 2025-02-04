<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import type { NodeLocator } from '$lib';

	interface Props {
		parentNodeLocator: NodeLocator;
		prompt: string;
		onCreateNode: (text: string) => void;
	}
	let text = $state('<think>\n</think>\n<actions>\n</actions>');
	let visibleText = $derived(text.replaceAll(' ', '·').replaceAll('\t', ' - '));
	const props: Props = $props();

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			// too easy to accidentally press escape while working on a big item
			event.stopPropagation();
			return;
		}
		if (event.key === 'Tab') {
			event.preventDefault();

			const textarea = event.target as HTMLTextAreaElement;
			const start = textarea.selectionStart;
			const end = textarea.selectionEnd;

			text = text.substring(0, start) + '\t' + text.substring(end);

			setTimeout(() => {
				textarea.selectionStart = textarea.selectionEnd = start + 1;
			}, 0);
		}
	}
	let open = $state(false);
</script>

<Dialog.Root bind:open>
	<Dialog.Trigger class={buttonVariants({ variant: 'outline' })}>➕ Child</Dialog.Trigger>
	<Dialog.Content class="min-h-[80vh] min-w-[90vw]">
		<Dialog.Header>
			<Dialog.Title>Create a new node</Dialog.Title>
			<Dialog.Description>Create a new node as a child of the selected node.</Dialog.Description>
		</Dialog.Header>
		<div class="flex flex-col gap-2">
			<p class="text-sm font-medium">Prompt:</p>
			<pre class="max-h-[200px] overflow-y-auto text-wrap text-xs">{props.prompt}</pre>
		</div>
		<Textarea bind:value={text} onkeydown={handleKeydown} />
		<pre class="max-h-[800px] overflow-y-auto text-wrap text-xs">{visibleText}</pre>
		<Dialog.Footer>
			<Button
				type="submit"
				onclick={() => {
					props.onCreateNode(text);
					open = false;
				}}>🆗 Create</Button
			>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
