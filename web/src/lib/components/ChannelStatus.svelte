<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type ChannelStatus, type ChannelStatusResponse } from '$lib/api';

	let status: ChannelStatusResponse | null = null;
	let loading = true;
	let error = '';
	let copiedCommand: string | null = null;

	onMount(() => {
		loadStatus();
	});

	async function loadStatus() {
		loading = true;
		try {
			status = await api.getChannelStatus();
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load channel status';
		} finally {
			loading = false;
		}
	}

	function getStatusIcon(channel: ChannelStatus): string {
		if (channel.ready) return '✅';
		if (channel.configuredInOC && channel.storedCreds.length > 0) return '🔶';
		if (channel.configuredInOC) return '⚠️';
		return '❌';
	}

	function getStatusText(channel: ChannelStatus): string {
		if (channel.ready) return 'Ready';
		if (channel.configuredInOC && channel.storedCreds.length > 0) {
			const missing = channel.requiredCreds.filter(c => !channel.storedCreds.includes(c));
			return `Missing: ${missing.join(', ')}`;
		}
		if (channel.configuredInOC) return 'Needs credentials in OCM';
		return 'Not configured in OpenClaw';
	}

	function getStatusClass(channel: ChannelStatus): string {
		if (channel.ready) return 'bg-green-50 border-green-200';
		if (channel.configuredInOC) return 'bg-yellow-50 border-yellow-200';
		return 'bg-gray-50 border-gray-200';
	}

	async function copyCommand(command: string) {
		try {
			await navigator.clipboard.writeText(command);
			copiedCommand = command;
			setTimeout(() => copiedCommand = null, 2000);
		} catch {
			// Fallback for non-secure contexts
			const textarea = document.createElement('textarea');
			textarea.value = command;
			document.body.appendChild(textarea);
			textarea.select();
			document.execCommand('copy');
			document.body.removeChild(textarea);
			copiedCommand = command;
			setTimeout(() => copiedCommand = null, 2000);
		}
	}
</script>

<div class="card">
	<div class="flex items-center justify-between mb-4">
		<h2 class="text-lg font-semibold text-gray-900">Channel Configuration</h2>
		<button 
			on:click={loadStatus} 
			class="text-sm text-gray-500 hover:text-gray-700"
			disabled={loading}
		>
			{loading ? 'Loading...' : 'Refresh'}
		</button>
	</div>

	{#if error}
		<div class="text-red-600 text-sm mb-4">{error}</div>
	{/if}

	{#if !status?.gatewayConnected}
		<div class="bg-amber-50 border border-amber-200 rounded-lg p-3 mb-4">
			<div class="flex items-center gap-2">
				<span>⚠️</span>
				<span class="text-amber-800 text-sm">
					Not connected to OpenClaw Gateway. Channel status may be incomplete.
				</span>
			</div>
		</div>
	{/if}

	{#if status}
		<div class="space-y-3">
			{#each status.channels as channel}
				<div class="border rounded-lg p-3 {getStatusClass(channel)}">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							<span class="text-lg">{getStatusIcon(channel)}</span>
							<span class="font-medium text-gray-900">{channel.label}</span>
						</div>
						<span class="text-sm text-gray-600">{getStatusText(channel)}</span>
					</div>

					{#if !channel.configuredInOC && channel.setupCommand}
						<div class="mt-3 bg-gray-900 rounded p-2">
							<div class="flex items-center justify-between">
								<code class="text-green-400 text-xs font-mono break-all">
									{channel.setupCommand}
								</code>
								<button
									on:click={() => copyCommand(channel.setupCommand || '')}
									class="ml-2 text-gray-400 hover:text-white text-xs flex-shrink-0"
								>
									{copiedCommand === channel.setupCommand ? '✓ Copied' : 'Copy'}
								</button>
							</div>
						</div>
						<p class="text-xs text-gray-500 mt-2">
							Run this command to configure {channel.label} in OpenClaw, then add credentials here.
						</p>
					{:else if channel.configuredInOC && !channel.ready}
						<div class="mt-2">
							<p class="text-xs text-gray-600">
								Required credentials: 
								{#each channel.requiredCreds as cred, i}
									<code class="bg-gray-100 px-1 rounded text-gray-800">{cred}</code>{i < channel.requiredCreds.length - 1 ? ', ' : ''}
								{/each}
							</p>
							{#if channel.storedCreds.length > 0}
								<p class="text-xs text-green-600 mt-1">
									✓ Stored: {channel.storedCreds.join(', ')}
								</p>
							{/if}
							<a 
								href="/credentials" 
								class="inline-block mt-2 text-sm text-blue-600 hover:text-blue-800"
							>
								→ Add credentials
							</a>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{:else if loading}
		<div class="flex items-center justify-center py-8">
			<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-primary-600"></div>
		</div>
	{/if}
</div>
