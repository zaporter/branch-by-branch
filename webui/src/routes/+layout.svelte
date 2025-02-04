<script lang="ts">
	import { page } from '$app/stores';
	import '../app.css';
	import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';

	let { children } = $props();

	const queryClient = new QueryClient();
</script>

<div class="flex h-screen max-h-screen flex-col">
	<nav class="flex flex-row items-center justify-around gap-4 bg-gray-200">
		<a
			href="/graph"
			class={`h-full w-full p-4 text-center ${$page.url.pathname === '/graph' ? 'bg-gray-300' : ''}`}
			>Graph</a
		>
		<div class="h-6 border-l border-gray-400"></div>
		<a
			href="/control"
			class={`h-full w-full p-4 text-center ${$page.url.pathname === '/control' ? 'bg-gray-300' : ''}`}
			>Control</a
		>
		<div class="h-6 border-l border-gray-400"></div>
		<a
			href="/status"
			class={`h-full w-full p-4 text-center ${$page.url.pathname === '/status' ? 'bg-gray-300' : ''}`}
			>Status</a
		>
	</nav>
	<!-- fix this-->
	<div class="max-h-[calc(100vh-50px)]">
		<QueryClientProvider client={queryClient}>
			{@render children()}
		</QueryClientProvider>
	</div>
</div>
