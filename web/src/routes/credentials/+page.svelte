<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Credential } from '$lib/api';

	let credentials: Credential[] = [];
	let loading = true;
	let error = '';
	let showAddModal = false;

	onMount(async () => {
		await loadCredentials();
	});

	async function loadCredentials() {
		loading = true;
		error = '';
		try {
			credentials = await api.listCredentials();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load credentials';
		} finally {
			loading = false;
		}
	}

	async function deleteCredential(service: string) {
		if (!confirm(`Delete credential "${service}"? This cannot be undone.`)) return;
		
		try {
			await api.deleteCredential(service);
			await loadCredentials();
		} catch (e) {
			alert(e instanceof Error ? e.message : 'Failed to delete');
		}
	}

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}

	function formatTtl(seconds: number): string {
		if (!seconds) return 'N/A';
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
		return `${Math.floor(seconds / 3600)}h`;
	}
</script>

<svelte:head>
	<title>Credentials - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Credentials</h1>
		<button class="btn btn-primary" on:click={() => showAddModal = true}>
			Add Credential
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
	{:else if credentials.length === 0}
		<div class="card p-12 text-center">
			<div class="text-4xl mb-4">🔑</div>
			<h3 class="text-lg font-medium text-gray-900">No credentials yet</h3>
			<p class="mt-2 text-sm text-gray-500">Add your first credential to get started.</p>
			<button class="btn btn-primary mt-4" on:click={() => showAddModal = true}>
				Add Credential
			</button>
		</div>
	{:else}
		<div class="card overflow-hidden">
			<table class="min-w-full divide-y divide-gray-200">
				<thead class="bg-gray-50">
					<tr>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Service
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Type
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Scopes
						</th>
						<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
							Updated
						</th>
						<th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
							Actions
						</th>
					</tr>
				</thead>
				<tbody class="bg-white divide-y divide-gray-200">
					{#each credentials as cred}
						<tr class="hover:bg-gray-50">
							<td class="px-6 py-4 whitespace-nowrap">
								<div class="text-sm font-medium text-gray-900">{cred.displayName}</div>
								<div class="text-xs text-gray-500">{cred.service}</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap">
								<span class="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-700 rounded">
									{cred.type}
								</span>
							</td>
							<td class="px-6 py-4">
								<div class="flex flex-wrap gap-1">
									{#each Object.entries(cred.scopes) as [name, scope]}
										<span class="px-2 py-0.5 text-xs rounded {scope.permanent ? 'bg-green-100 text-green-700' : 'bg-orange-100 text-orange-700'}">
											{name}
											{#if !scope.permanent}
												(approval)
											{/if}
										</span>
									{/each}
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(cred.updatedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
								<a href="/credentials/{cred.service}" class="text-primary-600 hover:text-primary-900 mr-4">
									Edit
								</a>
								<button
									class="text-red-600 hover:text-red-900"
									on:click={() => deleteCredential(cred.service)}
								>
									Delete
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<!-- TODO: Add modal for creating credentials -->
