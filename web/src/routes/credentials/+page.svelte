<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Credential } from '$lib/api';
	import { serviceTemplates, categoryLabels, getTemplateById, type ServiceTemplate } from '$lib/serviceTemplates';

	let credentials: Credential[] = [];
	let loading = true;
	let error = '';
	
	// Modal state
	let showAddModal = false;
	let modalStep: 'select' | 'configure' = 'select';
	let selectedTemplate: ServiceTemplate | null = null;
	let isCustom = false;
	let saving = false;
	let saveError = '';

	// Form state for template-based
	let fieldValues: Record<string, string> = {};
	let elevationMode: 'permanent' | 'approval' = 'permanent';
	let defaultTTL = '1h';

	// Form state for custom
	let customService = '';
	let customDisplayName = '';
	let customEnvVar = '';
	let customToken = '';

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

	function openModal() {
		showAddModal = true;
		modalStep = 'select';
		selectedTemplate = null;
		isCustom = false;
		fieldValues = {};
		saveError = '';
	}

	function closeModal() {
		showAddModal = false;
		modalStep = 'select';
		selectedTemplate = null;
		isCustom = false;
		fieldValues = {};
		customService = '';
		customDisplayName = '';
		customEnvVar = '';
		customToken = '';
		saveError = '';
	}

	function selectTemplate(template: ServiceTemplate) {
		selectedTemplate = template;
		isCustom = false;
		modalStep = 'configure';
		fieldValues = {};
		// Set default elevation based on template
		if (template.elevationConfig?.readOnly) {
			elevationMode = 'permanent';
		} else {
			elevationMode = 'approval';
			defaultTTL = template.elevationConfig?.defaultTTL || '1h';
		}
	}

	function selectCustom() {
		selectedTemplate = null;
		isCustom = true;
		modalStep = 'configure';
		elevationMode = 'approval';
		defaultTTL = '1h';
	}

	function goBack() {
		modalStep = 'select';
		selectedTemplate = null;
		isCustom = false;
	}

	async function saveCredential() {
		saving = true;
		saveError = '';

		try {
			if (isCustom) {
				// Custom credential
				if (!customService || !customDisplayName || !customEnvVar) {
					saveError = 'Service ID, Display Name, and Env Var are required';
					return;
				}

				await api.createCredential({
					service: customService,
					displayName: customDisplayName,
					type: 'custom',
					scopes: {
						default: {
							envVar: customEnvVar,
							token: customToken,
							permanent: elevationMode === 'permanent',
							requiresApproval: elevationMode === 'approval',
							maxTTL: defaultTTL
						}
					}
				});
			} else if (selectedTemplate) {
				// Template-based credential
				const missingFields = selectedTemplate.fields
					.filter(f => f.required && !fieldValues[f.name])
					.map(f => f.label);

				if (missingFields.length > 0) {
					saveError = `Required fields: ${missingFields.join(', ')}`;
					return;
				}

				// Build scopes from template fields
				const scopes: Record<string, any> = {};
				for (const field of selectedTemplate.fields) {
					if (fieldValues[field.name]) {
						scopes[field.name] = {
							envVar: field.envVar,
							token: fieldValues[field.name],
							permanent: elevationMode === 'permanent',
							requiresApproval: elevationMode === 'approval',
							maxTTL: defaultTTL
						};
					}
				}

				await api.createCredential({
					service: selectedTemplate.id,
					displayName: selectedTemplate.name,
					type: selectedTemplate.category,
					scopes
				});
			}

			closeModal();
			await loadCredentials();
		} catch (e) {
			saveError = e instanceof Error ? e.message : 'Failed to save credential';
		} finally {
			saving = false;
		}
	}

	// Group templates by category
	$: groupedTemplates = Object.entries(categoryLabels).map(([key, label]) => ({
		category: key,
		label,
		templates: serviceTemplates.filter(t => t.category === key)
	}));
</script>

<svelte:head>
	<title>Credentials - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Credentials</h1>
		<button class="btn btn-primary" on:click={openModal}>
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
			<button class="btn btn-primary mt-4" on:click={openModal}>
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
							Fields
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
									{#each Object.entries(cred.scopes || {}) as [name, scope]}
										<span class="px-2 py-0.5 text-xs rounded {scope.permanent ? 'bg-green-100 text-green-700' : 'bg-orange-100 text-orange-700'}">
											{scope.envVar || name}
											{#if !scope.permanent}
												⚡
											{/if}
										</span>
									{/each}
								</div>
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
								{formatDate(cred.updatedAt)}
							</td>
							<td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
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
			<!-- Header -->
			<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
				<div class="flex items-center gap-3">
					{#if modalStep === 'configure'}
						<button class="text-gray-400 hover:text-gray-600" on:click={goBack} aria-label="Go back">
							<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
							</svg>
						</button>
					{/if}
					<h2 class="text-lg font-semibold text-gray-900">
						{#if modalStep === 'select'}
							Add Credential
						{:else if isCustom}
							Custom Credential
						{:else if selectedTemplate}
							{selectedTemplate.name}
						{/if}
					</h2>
				</div>
				<button class="text-gray-400 hover:text-gray-600" on:click={closeModal} aria-label="Close">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="p-6">
				{#if modalStep === 'select'}
					<!-- Service Selection -->
					<div class="space-y-6">
						<!-- Docs link -->
						<div class="p-3 bg-gray-50 border border-gray-200 rounded-lg flex items-center justify-between">
							<span class="text-sm text-gray-600">
								Looking for a different integration?
							</span>
							<a 
								href="https://docs.openclaw.ai/channels" 
								target="_blank" 
								rel="noopener"
								class="text-sm text-primary-600 hover:text-primary-700 font-medium flex items-center gap-1"
							>
								View all OpenClaw integrations
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
								</svg>
							</a>
						</div>

						{#each groupedTemplates as group}
							<div>
								<h3 class="text-sm font-medium text-gray-500 mb-3">{group.label}</h3>
								<div class="grid grid-cols-2 gap-2">
									{#each group.templates as template}
										<button
											class="flex items-center gap-3 p-3 border border-gray-200 rounded-lg hover:border-primary-500 hover:bg-primary-50 text-left transition-colors"
											on:click={() => selectTemplate(template)}
										>
											<div class="flex-1">
												<div class="font-medium text-gray-900">{template.name}</div>
												<div class="text-xs text-gray-500">{template.description}</div>
											</div>
											<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
											</svg>
										</button>
									{/each}
								</div>
							</div>
						{/each}

						<!-- Custom option -->
						<div>
							<h3 class="text-sm font-medium text-gray-500 mb-3">Other</h3>
							<button
								class="flex items-center gap-3 p-3 border border-gray-200 rounded-lg hover:border-primary-500 hover:bg-primary-50 text-left transition-colors w-full"
								on:click={selectCustom}
							>
								<div class="flex-1">
									<div class="font-medium text-gray-900">Custom Credential</div>
									<div class="text-xs text-gray-500">For services not listed above</div>
								</div>
								<svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
								</svg>
							</button>
						</div>
					</div>

				{:else if modalStep === 'configure'}
					<!-- Configuration Form -->
					<div class="space-y-6">
						{#if saveError}
							<div class="p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
								{saveError}
							</div>
						{/if}

						{#if selectedTemplate}
							<!-- Template-based form -->
							{#if selectedTemplate.docsUrl}
								<div class="p-3 bg-blue-50 border border-blue-200 rounded">
									<p class="text-sm text-blue-700">
										📖 <a href={selectedTemplate.docsUrl} target="_blank" rel="noopener" class="underline hover:no-underline">
											View setup documentation
										</a>
									</p>
								</div>
							{/if}

							{#each selectedTemplate.fields as field}
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1" for={field.name}>
										{field.label}
										{#if field.required}<span class="text-red-500">*</span>{/if}
									</label>
									{#if field.type === 'textarea'}
										<textarea
											id={field.name}
											bind:value={fieldValues[field.name]}
											placeholder={field.placeholder}
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
											rows="3"
										></textarea>
									{:else}
										<input
											id={field.name}
											type={field.type}
											bind:value={fieldValues[field.name]}
											placeholder={field.placeholder}
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
										/>
									{/if}
									{#if field.helpText}
										<p class="mt-1 text-xs text-gray-500">{field.helpText}</p>
									{/if}
								</div>
							{/each}

						{:else if isCustom}
							<!-- Custom form -->
							<div class="grid grid-cols-2 gap-4">
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1" for="customService">
										Service ID <span class="text-red-500">*</span>
									</label>
									<input
										id="customService"
										type="text"
										bind:value={customService}
										placeholder="my-service"
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
									/>
								</div>
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1" for="customDisplayName">
										Display Name <span class="text-red-500">*</span>
									</label>
									<input
										id="customDisplayName"
										type="text"
										bind:value={customDisplayName}
										placeholder="My Service"
										class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
									/>
								</div>
							</div>

							<div>
								<label class="block text-sm font-medium text-gray-700 mb-1" for="customEnvVar">
									Environment Variable <span class="text-red-500">*</span>
								</label>
								<input
									id="customEnvVar"
									type="text"
									bind:value={customEnvVar}
									placeholder="MY_SERVICE_API_KEY"
									class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
								/>
								<p class="mt-1 text-xs text-gray-500">The env var name that will be injected into the Gateway</p>
							</div>

							<div>
								<label class="block text-sm font-medium text-gray-700 mb-1" for="customToken">
									Token/Secret
								</label>
								<input
									id="customToken"
									type="password"
									bind:value={customToken}
									placeholder="Enter token or secret..."
									class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
								/>
							</div>
						{/if}

						<!-- Elevation Settings -->
						<div class="border-t border-gray-200 pt-6">
							<h3 class="text-sm font-medium text-gray-700 mb-3">Access Control</h3>
							
							<div class="space-y-3">
								<label class="flex items-start gap-3 p-3 border border-gray-200 rounded-lg cursor-pointer hover:bg-gray-50">
									<input
										type="radio"
										name="elevationMode"
										value="permanent"
										bind:group={elevationMode}
										class="mt-0.5"
									/>
									<div>
										<div class="font-medium text-gray-900">Permanent Access</div>
										<div class="text-sm text-gray-500">Agent can always use this credential (good for read-only APIs)</div>
									</div>
								</label>

								<label class="flex items-start gap-3 p-3 border border-gray-200 rounded-lg cursor-pointer hover:bg-gray-50">
									<input
										type="radio"
										name="elevationMode"
										value="approval"
										bind:group={elevationMode}
										class="mt-0.5"
									/>
									<div class="flex-1">
										<div class="font-medium text-gray-900">Requires Approval</div>
										<div class="text-sm text-gray-500">Agent must request access, you approve with a time limit</div>
										{#if elevationMode === 'approval'}
											<div class="mt-2">
												<label class="text-xs text-gray-500" for="defaultTTL">Default TTL</label>
												<select id="defaultTTL" bind:value={defaultTTL} class="ml-2 text-sm border-gray-300 rounded">
													<option value="15m">15 minutes</option>
													<option value="30m">30 minutes</option>
													<option value="1h">1 hour</option>
													<option value="2h">2 hours</option>
													<option value="4h">4 hours</option>
													<option value="8h">8 hours</option>
													<option value="24h">24 hours</option>
												</select>
											</div>
										{/if}
									</div>
								</label>
							</div>
						</div>
					</div>
				{/if}
			</div>

			<!-- Footer -->
			{#if modalStep === 'configure'}
				<div class="px-6 py-4 border-t border-gray-200 flex justify-end gap-3">
					<button class="btn btn-secondary" on:click={closeModal}>Cancel</button>
					<button class="btn btn-primary" on:click={saveCredential} disabled={saving}>
						{saving ? 'Saving...' : 'Save Credential'}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
