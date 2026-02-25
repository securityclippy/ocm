<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Credential } from '$lib/api';

	let credentials: Credential[] = [];
	let loading = true;
	let error = '';
	let showAddModal = false;
	let saving = false;
	let saveError = '';

	// Form state
	let newCred = {
		service: '',
		displayName: '',
		type: 'oauth2',
		scopes: [{ name: 'default', envVar: '', token: '', requiresApproval: true, permanent: false, maxTTL: '1h' }]
	};

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

	function resetForm() {
		newCred = {
			service: '',
			displayName: '',
			type: 'oauth2',
			scopes: [{ name: 'default', envVar: '', token: '', requiresApproval: true, permanent: false, maxTTL: '1h' }]
		};
		saveError = '';
	}

	function closeModal() {
		showAddModal = false;
		resetForm();
	}

	function addScope() {
		newCred.scopes = [...newCred.scopes, { name: '', envVar: '', token: '', requiresApproval: true, permanent: false, maxTTL: '1h' }];
	}

	function removeScope(index: number) {
		newCred.scopes = newCred.scopes.filter((_, i) => i !== index);
	}

	async function saveCredential() {
		if (!newCred.service || !newCred.displayName) {
			saveError = 'Service ID and Display Name are required';
			return;
		}

		// Convert scopes array to object
		const scopesObj: Record<string, any> = {};
		for (const scope of newCred.scopes) {
			if (!scope.name) continue;
			scopesObj[scope.name] = {
				envVar: scope.envVar,
				token: scope.token,
				requiresApproval: scope.requiresApproval,
				permanent: scope.permanent,
				maxTTL: scope.maxTTL
			};
		}

		saving = true;
		saveError = '';
		try {
			await api.createCredential({
				service: newCred.service,
				displayName: newCred.displayName,
				type: newCred.type,
				scopes: scopesObj
			});
			closeModal();
			await loadCredentials();
		} catch (e) {
			saveError = e instanceof Error ? e.message : 'Failed to save credential';
		} finally {
			saving = false;
		}
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

<!-- Add Credential Modal -->
{#if showAddModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
		<div class="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
			<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
				<h2 class="text-lg font-semibold text-gray-900">Add Credential</h2>
				<button class="text-gray-400 hover:text-gray-600" on:click={closeModal}>
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="p-6 space-y-4">
				{#if saveError}
					<div class="p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
						{saveError}
					</div>
				{/if}

				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Service ID</label>
						<input
							type="text"
							bind:value={newCred.service}
							placeholder="gmail, github, etc."
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
						/>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Display Name</label>
						<input
							type="text"
							bind:value={newCred.displayName}
							placeholder="Gmail API, GitHub, etc."
							class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
						/>
					</div>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Type</label>
					<select
						bind:value={newCred.type}
						class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
					>
						<option value="oauth2">OAuth2</option>
						<option value="api_key">API Key</option>
						<option value="token">Token</option>
						<option value="password">Username/Password</option>
					</select>
				</div>

				<div>
					<div class="flex items-center justify-between mb-2">
						<label class="block text-sm font-medium text-gray-700">Scopes</label>
						<button type="button" class="text-sm text-primary-600 hover:text-primary-700" on:click={addScope}>
							+ Add Scope
						</button>
					</div>

					<div class="space-y-3">
						{#each newCred.scopes as scope, i}
							<div class="p-4 bg-gray-50 rounded-lg space-y-3">
								<div class="flex items-center justify-between">
									<span class="text-sm font-medium text-gray-600">Scope {i + 1}</span>
									{#if newCred.scopes.length > 1}
										<button type="button" class="text-red-500 hover:text-red-700 text-sm" on:click={() => removeScope(i)}>
											Remove
										</button>
									{/if}
								</div>

								<div class="grid grid-cols-2 gap-3">
									<div>
										<label class="block text-xs text-gray-500 mb-1">Scope Name</label>
										<input
											type="text"
											bind:value={scope.name}
											placeholder="read, write, send, etc."
											class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
										/>
									</div>
									<div>
										<label class="block text-xs text-gray-500 mb-1">Environment Variable</label>
										<input
											type="text"
											bind:value={scope.envVar}
											placeholder="GMAIL_TOKEN"
											class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
										/>
									</div>
								</div>

								<div>
									<label class="block text-xs text-gray-500 mb-1">Token/Secret</label>
									<input
										type="password"
										bind:value={scope.token}
										placeholder="Enter token or secret..."
										class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
									/>
								</div>

								<div class="grid grid-cols-3 gap-3">
									<div>
										<label class="block text-xs text-gray-500 mb-1">Max TTL</label>
										<select bind:value={scope.maxTTL} class="w-full px-3 py-2 border border-gray-300 rounded-md text-sm">
											<option value="15m">15 minutes</option>
											<option value="30m">30 minutes</option>
											<option value="1h">1 hour</option>
											<option value="2h">2 hours</option>
											<option value="4h">4 hours</option>
											<option value="8h">8 hours</option>
											<option value="24h">24 hours</option>
										</select>
									</div>
									<div class="flex items-center pt-5">
										<label class="flex items-center gap-2 text-sm">
											<input type="checkbox" bind:checked={scope.requiresApproval} class="rounded" />
											Requires Approval
										</label>
									</div>
									<div class="flex items-center pt-5">
										<label class="flex items-center gap-2 text-sm">
											<input type="checkbox" bind:checked={scope.permanent} class="rounded" />
											Permanent Access
										</label>
									</div>
								</div>
							</div>
						{/each}
					</div>
				</div>
			</div>

			<div class="px-6 py-4 border-t border-gray-200 flex justify-end gap-3">
				<button class="btn btn-secondary" on:click={closeModal}>Cancel</button>
				<button class="btn btn-primary" on:click={saveCredential} disabled={saving}>
					{saving ? 'Saving...' : 'Save Credential'}
				</button>
			</div>
		</div>
	</div>
{/if}
