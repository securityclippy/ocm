<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AuditEntry } from '$lib/api';

	let entries: AuditEntry[] = [];
	let loading = true;
	let error = '';
	let serviceFilter = '';

	onMount(async () => {
		await loadAudit();
	});

	async function loadAudit() {
		loading = true;
		error = '';
		try {
			entries = await api.listAuditEntries(serviceFilter || undefined);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load audit log';
		} finally {
			loading = false;
		}
	}

	function formatTime(iso: string): string {
		return new Date(iso).toLocaleString();
	}

	function getActionColor(action: string): string {
		if (action.includes('approved')) return 'text-green-600 bg-green-50';
		if (action.includes('denied') || action.includes('revoked')) return 'text-red-600 bg-red-50';
		if (action.includes('request')) return 'text-orange-600 bg-orange-50';
		if (action.includes('access')) return 'text-blue-600 bg-blue-50';
		if (action.includes('created')) return 'text-purple-600 bg-purple-50';
		if (action.includes('deleted')) return 'text-red-600 bg-red-50';
		return 'text-gray-600 bg-gray-50';
	}

	function formatAction(action: string): string {
		return action.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
	}
</script>

<svelte:head>
	<title>Audit Log - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Audit Log</h1>
		<div class="flex items-center gap-4">
			<input
				type="text"
				bind:value={serviceFilter}
				placeholder="Filter by service..."
				class="input w-48"
				on:keydown={(e) => e.key === 'Enter' && loadAudit()}
			/>
			<button class="btn btn-secondary" on:click={loadAudit}>
				Refresh
			</button>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
		</div>
	{:else if error}
		<div class="card p-4 bg-red-50 border-red-200">
			<p class="text-red-700">{error}</p>
		</div>
	{:else if entries.length === 0}
		<div class="card p-12 text-center">
			<div class="text-4xl mb-4">📜</div>
			<h3 class="text-lg font-medium text-gray-900">No audit entries</h3>
			<p class="mt-2 text-sm text-gray-500">Activity will appear here as it happens.</p>
		</div>
	{:else}
		<div class="card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Timestamp
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Action
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Service / Scope
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Details
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actor
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each entries as entry}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatTime(entry.timestamp)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs font-medium rounded {getActionColor(entry.action)}">
									{formatAction(entry.action)}
								</span>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm">
								{#if entry.service}
									<span class="font-medium text-gray-900">{entry.service}</span>
									{#if entry.scope}
										<span class="text-gray-500">:{entry.scope}</span>
									{/if}
								{:else}
									<span class="text-gray-400">—</span>
								{/if}
							</td>
							<td class="px-6 py-4 text-sm text-gray-500 max-w-xs truncate">
								{entry.details || '—'}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{entry.actor}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
