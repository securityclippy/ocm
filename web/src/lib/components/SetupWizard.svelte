<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { api } from '$lib/api';
	import { serviceTemplates, type ServiceTemplate } from '$lib/serviceTemplates';

	const dispatch = createEventDispatcher();

	let step = 1;
	let selectedProvider: string | null = null;
	let apiKey = '';
	let envVar = '';
	let isCreating = false;
	let isCompleting = false;
	let error = '';

	// Model providers (required - at least one)
	const modelProviderIds = ['anthropic', 'openai', 'openrouter', 'groq'];
	const providerTemplates = serviceTemplates.filter((t) => modelProviderIds.includes(t.id));

	// Icon mapping since templates might not have icons
	const icons: Record<string, string> = {
		anthropic: '🧠',
		openai: '🤖',
		openrouter: '🔀',
		groq: '⚡',
		google: '🔍'
	};

	function selectProvider(id: string) {
		selectedProvider = id;
		const template = providerTemplates.find((t) => t.id === id);
		if (template?.fields?.[0]?.envVar) {
			envVar = template.fields[0].envVar;
		}
		error = '';
	}

	function getTemplate(id: string): ServiceTemplate | undefined {
		return serviceTemplates.find((t) => t.id === id);
	}

	async function createCredential() {
		if (!selectedProvider || !apiKey.trim()) {
			error = 'Please enter your API key';
			return;
		}

		if (!envVar.trim()) {
			error = 'Environment variable is required';
			return;
		}

		isCreating = true;
		error = '';

		try {
			const template = getTemplate(selectedProvider);
			// For LLM providers, we only need a "read" credential (always available)
			// No readWrite needed - API keys don't have read vs write distinction
			await api.createCredential({
				service: selectedProvider,
				displayName: template?.name || selectedProvider,
				type: 'api_key',
				read: {
					envVar: envVar.trim(),
					token: apiKey.trim()
				}
				// No readWrite - LLM API keys are always full access
			});
			step = 3;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save credential';
		} finally {
			isCreating = false;
		}
	}

	async function completeSetup() {
		isCompleting = true;
		error = '';

		try {
			await api.completeSetup();
			dispatch('complete');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to complete setup';
		} finally {
			isCompleting = false;
		}
	}
</script>

<div class="min-h-screen bg-gray-900 flex items-center justify-center p-4">
	<div class="max-w-2xl w-full">
		<!-- Header -->
		<div class="text-center mb-8">
			<div class="text-5xl mb-4">🔐</div>
			<h1 class="text-3xl font-bold text-white mb-2">OCM Setup</h1>
			<p class="text-gray-400">OpenClaw Credential Manager</p>
		</div>

		<!-- Progress -->
		<div class="flex justify-center gap-2 mb-8">
			{#each [1, 2, 3] as s}
				<div
					class="w-3 h-3 rounded-full transition-colors {s === step
						? 'bg-blue-500'
						: s < step
							? 'bg-green-500'
							: 'bg-gray-600'}"
				/>
			{/each}
		</div>

		<!-- Card -->
		<div class="bg-gray-800 rounded-lg shadow-xl border border-gray-700">
			{#if step === 1}
				<!-- Step 1: Welcome -->
				<div class="p-8">
					<h2 class="text-xl font-semibold text-white mb-4">Welcome to OCM</h2>
					<p class="text-gray-300 mb-6">
						OCM securely manages credentials for your OpenClaw instance. Your API keys are encrypted
						and only injected into OpenClaw when needed.
					</p>

					<div class="bg-gray-700/50 rounded-lg p-4 mb-6">
						<h3 class="text-sm font-medium text-gray-300 mb-2">What you'll need:</h3>
						<ul class="text-sm text-gray-400 space-y-1">
							<li>• An API key from a model provider (Anthropic, OpenAI, or Google)</li>
							<li>• Optional: API keys for other services (Slack, Notion, etc.)</li>
						</ul>
					</div>

					<button
						on:click={() => (step = 2)}
						class="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-3 px-4 rounded-lg transition-colors"
					>
						Get Started →
					</button>
				</div>
			{:else if step === 2}
				<!-- Step 2: Model Provider -->
				<div class="p-8">
					<h2 class="text-xl font-semibold text-white mb-2">Configure Model Provider</h2>
					<p class="text-gray-400 mb-6">
						OpenClaw needs an LLM API key to work. Select your provider:
					</p>

					<!-- Provider Selection -->
					<div class="grid grid-cols-2 gap-3 mb-6">
						{#each providerTemplates as provider}
							<button
								on:click={() => selectProvider(provider.id)}
								class="p-4 rounded-lg border-2 text-left transition-all {selectedProvider ===
								provider.id
									? 'border-blue-500 bg-blue-500/10'
									: 'border-gray-600 hover:border-gray-500 bg-gray-700/50'}"
							>
								<div class="text-2xl mb-1">{icons[provider.id] || '🔑'}</div>
								<div class="font-medium text-white">{provider.name}</div>
								<div class="text-xs text-gray-400">{provider.description}</div>
							</button>
						{/each}
					</div>

					<!-- API Key Input -->
					{#if selectedProvider}
						{@const template = getTemplate(selectedProvider)}
						<div class="space-y-4">
							{#if template?.setupInstructions}
								<div class="bg-amber-900/30 border border-amber-700/50 rounded-lg p-4">
									<h4 class="text-sm font-medium text-amber-400 mb-2">How to get your API key:</h4>
									<div class="text-sm text-amber-200/80 whitespace-pre-line">
										{template.setupInstructions}
									</div>
									{#if template?.docsUrl}
										<a
											href={template.docsUrl}
											target="_blank"
											rel="noopener"
											class="inline-block mt-2 text-sm text-amber-400 hover:text-amber-300"
										>
											📖 Full documentation →
										</a>
									{/if}
								</div>
							{/if}

							<div>
								<label for="apiKey" class="block text-sm font-medium text-gray-300 mb-1">
									API Key
								</label>
								<input
									id="apiKey"
									type="password"
									bind:value={apiKey}
									placeholder="sk-..."
									class="w-full bg-gray-700 border border-gray-600 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
								/>
							</div>

							<div>
								<label for="envVar" class="block text-sm font-medium text-gray-300 mb-1">
									Environment Variable
								</label>
								<input
									id="envVar"
									type="text"
									bind:value={envVar}
									class="w-full bg-gray-700 border border-gray-600 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
								/>
								<p class="text-xs text-gray-500 mt-1">
									OpenClaw will see this as ${envVar}
								</p>
							</div>

							{#if error}
								<div class="text-red-400 text-sm">{error}</div>
							{/if}

							<button
								on:click={createCredential}
								disabled={isCreating || !apiKey.trim()}
								class="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-medium py-3 px-4 rounded-lg transition-colors"
							>
								{isCreating ? 'Saving...' : 'Save & Continue →'}
							</button>
						</div>
					{/if}

					<button on:click={() => (step = 1)} class="mt-4 text-sm text-gray-400 hover:text-gray-300">
						← Back
					</button>
				</div>
			{:else if step === 3}
				<!-- Step 3: Complete -->
				<div class="p-8 text-center">
					<div class="text-5xl mb-4">✅</div>
					<h2 class="text-xl font-semibold text-white mb-2">Ready to Go!</h2>
					<p class="text-gray-400 mb-6">
						Your model provider is configured. Click below to start OpenClaw with your credentials.
					</p>

					<div class="bg-gray-700/50 rounded-lg p-4 mb-6 text-left">
						<h3 class="text-sm font-medium text-gray-300 mb-2">What happens next:</h3>
						<ul class="text-sm text-gray-400 space-y-1">
							<li>1. OCM writes credentials to OpenClaw's environment</li>
							<li>2. OpenClaw restarts to load the new configuration</li>
							<li>3. You can add more services anytime from the dashboard</li>
						</ul>
					</div>

					{#if error}
						<div class="text-red-400 text-sm mb-4">{error}</div>
					{/if}

					<button
						on:click={completeSetup}
						disabled={isCompleting}
						class="w-full bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-medium py-3 px-4 rounded-lg transition-colors"
					>
						{isCompleting ? 'Starting OpenClaw...' : '🚀 Launch OpenClaw'}
					</button>

					<button
						on:click={() => (step = 2)}
						class="mt-4 text-sm text-gray-400 hover:text-gray-300"
					>
						← Add another provider
					</button>
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<p class="text-center text-gray-500 text-sm mt-6">
			OCM — OpenClaw Credential Manager
		</p>
	</div>
</div>
