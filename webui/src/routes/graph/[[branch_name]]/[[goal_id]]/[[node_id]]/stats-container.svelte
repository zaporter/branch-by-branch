<script lang="ts">
	import type { Snippet } from 'svelte';
	interface Props {
		title: string;
		children: Snippet;
	}
	const props: Props = $props();
	let isCollapsed = $state(false);
</script>

<div class="flex w-full flex-col gap-4 bg-gray-100 p-4">
	<div class="flex flex-row justify-between">
		<h2 class="text-md font-bold">{props.title}</h2>
		<button class="float-right" onclick={() => (isCollapsed = !isCollapsed)}>
			{isCollapsed ? '⬇️' : '⬆️'}
		</button>
	</div>
	{#if !isCollapsed}
		{@render props.children()}
	{/if}
</div>
