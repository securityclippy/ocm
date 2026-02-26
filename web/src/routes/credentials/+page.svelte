<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Credential } from '$lib/api';
	import { serviceTemplates, categoryLabels, getTemplateById, type ServiceTemplate } from '$lib/serviceTemplates';

	let credentials: Credential[] = [];
	let loading = true;
	let error = '';
	let configWarning = '';  // Warning about restart configuration
	
	// Modal state
	let showAddModal = false;
	let modalStep: 'select' | 'configure' = 'select';
	let selectedTemplate: ServiceTemplate | null = null;
	let isCustom = false;
	let saving = false;
	let saveError = '';

	// Form state for template-based
	let fieldValues: Record<string, string> = {};
	let defaultTTL = '1h';

	// Form state for custom
	let customService = '';
	let customDisplayName = '';
	let customEnvVar = '';
	let customReadToken = '';
	let customReadWriteToken = '';

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
		customReadToken = '';
		customReadWriteToken = '';
		defaultTTL = '1h';
		saveError = '';
	}

	function selectTemplate(template: ServiceTemplate) {
		selectedTemplate = template;
		isCustom = false;
		modalStep = 'configure';
		fieldValues = {};
		defaultTTL = template.elevationConfig?.defaultTTL || '1h';
	}

	function selectCustom() {
		selectedTemplate = null;
		isCustom = true;
		modalStep = 'configure';
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
				if (!customService || !customDisplayName || !customEnvVar || !customReadToken) {
					saveError = 'Service ID, Display Name, Env Var, and Read Key are required';
					return;
				}

				const request: any = {
					service: customService,
					displayName: customDisplayName,
					type: 'custom',
					read: {
						envVar: customEnvVar,
						token: customReadToken
					}
				};

				// Add readWrite only if provided
				if (customReadWriteToken) {
					request.readWrite = {
						envVar: customEnvVar,  // Same env var
						token: customReadWriteToken,
						maxTTL: defaultTTL
					};
				}

				const result = await api.createCredential(request);
				if (result?.warning) {
					configWarning = result.warning;
				}
			} else if (selectedTemplate) {
				// Template-based credential
				// Templates may use various field names: token, apiKey, botToken, readToken, etc.
				// Find the first required password field as the "read" token
				const tokenFields = selectedTemplate.fields.filter(f => f.type === 'password');
				const primaryTokenField = tokenFields.find(f => f.required) || tokenFields[0];
				
				if (!primaryTokenField || !fieldValues[primaryTokenField.name]) {
					saveError = `${primaryTokenField?.label || 'Token'} is required`;
					return;
				}

				// Use the primary token field's envVar
				const envVar = primaryTokenField.envVar;
				const readToken = fieldValues[primaryTokenField.name];

				// Check if there's an explicit readWriteToken field
				const readWriteField = tokenFields.find(f => f.name === 'readWriteToken');
				const readWriteToken = readWriteField ? fieldValues['readWriteToken'] : fieldValues['readWriteToken'];

				const request: any = {
					service: selectedTemplate.id,
					displayName: selectedTemplate.name,
					type: selectedTemplate.category,
					read: {
						envVar,
						token: readToken
					}
				};

				// Add readWrite if provided
				if (readWriteToken) {
					request.readWrite = {
						envVar,
						token: readWriteToken,
						maxTTL: defaultTTL
					};
				}

				const result = await api.createCredential(request);
				if (result?.warning) {
					configWarning = result.warning;
				}
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

	{#if configWarning}
		<div class="card p-4 bg-amber-50 border-amber-200">
			<div class="flex items-start gap-3">
				<span class="text-amber-500 text-xl">⚠️</span>
				<div class="flex-1">
					<h3 class="font-medium text-amber-800">Configuration Required</h3>
					<div class="mt-2 text-sm text-amber-700 whitespace-pre-wrap font-mono bg-amber-100 p-3 rounded">{configWarning}</div>
					<button 
						class="mt-3 text-sm text-amber-600 hover:text-amber-800 underline"
						on:click={() => configWarning = ''}
					>
						Dismiss
					</button>
				</div>
			</div>
		</div>
	{/if}

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
									{#if cred.read}
										<span class="px-2 py-0.5 text-xs rounded bg-green-100 text-green-700">
											{cred.read.envVar} (read)
										</span>
									{/if}
									{#if cred.readWrite}
										<span class="px-2 py-0.5 text-xs rounded bg-orange-100 text-orange-700">
											{cred.readWrite.envVar} (write) ⚡
										</span>
									{/if}
									{#if !cred.read && !cred.readWrite && cred.scopes}
										<!-- Legacy display -->
										{#each Object.entries(cred.scopes) as [name, scope]}
											<span class="px-2 py-0.5 text-xs rounded {scope.permanent ? 'bg-green-100 text-green-700' : 'bg-orange-100 text-orange-700'}">
												{scope.envVar || name}
												{#if !scope.permanent}⚡{/if}
											</span>
										{/each}
									{/if}
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

							{#if selectedTemplate.setupInstructions}
								<div class="p-4 bg-amber-50 border border-amber-200 rounded">
									<h4 class="text-sm font-medium text-amber-800 mb-2">⚙️ Setup Steps</h4>
									<pre class="text-xs text-amber-700 whitespace-pre-wrap font-mono">{selectedTemplate.setupInstructions}</pre>
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
									{:else if field.type === 'password'}
										<input
											id={field.name}
											type="password"
											bind:value={fieldValues[field.name]}
											placeholder={field.placeholder}
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
										/>
									{:else}
										<input
											id={field.name}
											type="text"
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

							<!-- Optional Read-Write Key for templates -->
							{#if !selectedTemplate.elevationConfig?.readOnly}
								<div class="border-t border-gray-200 pt-4 mt-4">
									<h4 class="text-sm font-medium text-gray-700 mb-2">Elevated Access (optional)</h4>
									<p class="text-xs text-gray-500 mb-3">
										If you have a separate key with write permissions, add it here. It will require approval before use.
									</p>
									
									<div>
										<label class="block text-sm font-medium text-gray-700 mb-1" for="readWriteToken">
											Read-Write Key <span class="text-gray-400 text-xs font-normal">(optional)</span>
										</label>
										<input
											id="readWriteToken"
											type="password"
											bind:value={fieldValues['readWriteToken']}
											placeholder="Full-access API key..."
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
										/>
										<p class="mt-1 text-xs text-gray-500">Replaces the read key when elevated (same env var)</p>
									</div>

									{#if fieldValues['readWriteToken']}
										<div class="mt-3 bg-amber-50 border border-amber-200 rounded-lg p-3">
											<div class="flex items-center gap-2 mb-2">
												<span class="text-amber-600">⚡</span>
												<span class="text-sm font-medium text-amber-800">Elevation Settings</span>
											</div>
											<div class="flex items-center gap-3">
												<label class="text-sm text-amber-700" for="templateTTL">Max elevation duration:</label>
												<select id="templateTTL" bind:value={defaultTTL} class="text-sm border-amber-300 rounded bg-white">
													<option value="15m">15 minutes</option>
													<option value="30m">30 minutes</option>
													<option value="1h">1 hour</option>
													<option value="2h">2 hours</option>
													<option value="4h">4 hours</option>
													<option value="8h">8 hours</option>
													<option value="24h">24 hours</option>
												</select>
											</div>
										</div>
									{/if}
								</div>
							{/if}

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

							<div class="border-t border-gray-200 pt-4 mt-4">
								<h4 class="text-sm font-medium text-gray-700 mb-3">Access Levels</h4>
								<p class="text-xs text-gray-500 mb-4">
									Provide a read-only key for normal access. Optionally add a read-write key that requires approval before use.
								</p>
								
								<div class="space-y-4">
									<div>
										<label class="block text-sm font-medium text-gray-700 mb-1" for="customReadToken">
											Read Key <span class="text-red-500">*</span>
										</label>
										<input
											id="customReadToken"
											type="password"
											bind:value={customReadToken}
											placeholder="Read-only API key or token..."
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
										/>
										<p class="mt-1 text-xs text-gray-500">Always available to the agent (permanent access)</p>
									</div>

									<div>
										<label class="block text-sm font-medium text-gray-700 mb-1" for="customReadWriteToken">
											Read-Write Key <span class="text-gray-400 text-xs font-normal">(optional)</span>
										</label>
										<input
											id="customReadWriteToken"
											type="password"
											bind:value={customReadWriteToken}
											placeholder="Full-access API key or token..."
											class="w-full px-3 py-2 border border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
										/>
										<p class="mt-1 text-xs text-gray-500">Requires your approval before use. Swapped in for the same env var.</p>
									</div>

									{#if customReadWriteToken}
										<div class="bg-amber-50 border border-amber-200 rounded-lg p-3">
											<div class="flex items-center gap-2 mb-2">
												<span class="text-amber-600">⚡</span>
												<span class="text-sm font-medium text-amber-800">Elevation Settings</span>
											</div>
											<div class="flex items-center gap-3">
												<label class="text-sm text-amber-700" for="defaultTTL">Max elevation duration:</label>
												<select id="defaultTTL" bind:value={defaultTTL} class="text-sm border-amber-300 rounded bg-white">
													<option value="15m">15 minutes</option>
													<option value="30m">30 minutes</option>
													<option value="1h">1 hour</option>
													<option value="2h">2 hours</option>
													<option value="4h">4 hours</option>
													<option value="8h">8 hours</option>
													<option value="24h">24 hours</option>
												</select>
											</div>
										</div>
									{/if}
								</div>
							</div>
						{/if}

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
