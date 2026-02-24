<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Elevation } from '$lib/api';
	import PendingRequests from '$lib/components/PendingRequests.svelte';

	let requests: Elevation[] = [];
	let loading = true;
	let error = '';

	onMount(async () => {
		await loadRequests();
	});

	async function loadRequests() {
		loading = true;
		error = '';
		try {
			requests = await api.listPendingRequests();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load requests';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Requests - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Elevation Requests</h1>
		<button class="btn btn-secondary" on:click={loadRequests}>
			Refresh
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
		</div>
	{:else if error}
		<div class="card p-4 bg-red-50 border-red-200">
			<p class="text-red-700">{error}</p>
		</div>
	{:else if requests.length === 0}
		<div class="card p-12 text-center">
			<div class="text-4xl mb-4">✅</div>
			<h3 class="text-lg font-medium text-gray-900">No pending requests</h3>
			<p class="mt-2 text-sm text-gray-500">All elevation requests have been handled.</p>
		</div>
	{:else}
		<PendingRequests {requests} on:action={loadRequests} />
	{/if}
</div>
