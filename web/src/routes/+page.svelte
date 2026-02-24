<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type DashboardData } from '$lib/api';
	import PendingRequests from '$lib/components/PendingRequests.svelte';
	import RecentActivity from '$lib/components/RecentActivity.svelte';

	let dashboard: DashboardData | null = null;
	let loading = true;
	let error = '';

	onMount(async () => {
		try {
			dashboard = await api.getDashboard();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load dashboard';
		} finally {
			loading = false;
		}
	});

	function refresh() {
		loading = true;
		error = '';
		api.getDashboard()
			.then(d => dashboard = d)
			.catch(e => error = e instanceof Error ? e.message : 'Failed to load')
			.finally(() => loading = false);
	}
</script>

<svelte:head>
	<title>Dashboard - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Dashboard</h1>
		<button class="btn btn-secondary" on:click={refresh}>
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
	{:else if dashboard}
		<!-- Stats -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
			<div class="card p-6">
				<div class="text-sm font-medium text-gray-500">Total Credentials</div>
				<div class="mt-2 text-3xl font-bold text-gray-900">{dashboard.totalCredentials}</div>
			</div>
			<div class="card p-6">
				<div class="text-sm font-medium text-gray-500">Pending Requests</div>
				<div class="mt-2 text-3xl font-bold text-orange-600">{dashboard.pendingRequests}</div>
			</div>
			<div class="card p-6">
				<div class="text-sm font-medium text-gray-500">Active Elevations</div>
				<div class="mt-2 text-3xl font-bold text-green-600">{dashboard.activeElevations}</div>
			</div>
		</div>

		<!-- Pending Requests -->
		{#if dashboard.pending && dashboard.pending.length > 0}
			<PendingRequests requests={dashboard.pending} on:action={refresh} />
		{/if}

		<!-- Recent Activity -->
		{#if dashboard.recentAudit && dashboard.recentAudit.length > 0}
			<RecentActivity entries={dashboard.recentAudit} />
		{/if}
	{/if}
</div>
