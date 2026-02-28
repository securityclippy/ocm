<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import SetupWizard from '$lib/components/SetupWizard.svelte';

	let setupComplete = true; // Default to true to avoid flash
	let loading = true;
	let gatewayStatus: { 
		connected: boolean; 
		pairingNeeded: boolean; 
		tokenMismatch?: boolean;
		deviceId?: string; 
		approveCommand?: string;
		fixCommand?: string;
	} | null = null;

	onMount(async () => {
		try {
			const status = await api.getSetupStatus();
			setupComplete = status.setupComplete;
			gatewayStatus = status.gatewayStatus || null;
		} catch (err) {
			console.error('Failed to check setup status:', err);
			// On error, assume setup is complete (show dashboard)
			setupComplete = true;
		} finally {
			loading = false;
		}
	});

	function handleSetupComplete() {
		setupComplete = true;
	}
	
	function copyCommand(cmd: string | undefined) {
		if (cmd) {
			navigator.clipboard.writeText(cmd);
		}
	}
</script>

{#if loading}
	<div class="min-h-screen bg-gray-900 flex items-center justify-center">
		<div class="text-gray-400">Loading...</div>
	</div>
{:else if !setupComplete}
	<SetupWizard on:complete={handleSetupComplete} />
{:else}
	<div class="min-h-screen flex">
		<Sidebar />
		<main class="flex-1 p-8">
			{#if gatewayStatus?.tokenMismatch}
				<div class="mb-6 bg-red-900/50 border border-red-600 rounded-lg p-4">
					<div class="flex items-start gap-3">
						<span class="text-2xl">🔑</span>
						<div class="flex-1">
							<h3 class="text-red-200 font-semibold">Gateway Token Mismatch</h3>
							<p class="text-red-100/80 text-sm mt-1">
								OCM and OpenClaw have different gateway tokens. Run this to sync them:
							</p>
							<div class="mt-3 bg-black/40 rounded p-3 font-mono text-sm text-green-400 relative group">
								<pre class="whitespace-pre-wrap">{gatewayStatus.fixCommand}</pre>
								<button 
									on:click={() => copyCommand(gatewayStatus?.fixCommand)}
									class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity bg-gray-700 hover:bg-gray-600 px-2 py-1 rounded text-xs text-gray-300"
								>
									Copy
								</button>
							</div>
							<p class="text-red-100/60 text-xs mt-2">
								This reads OpenClaw's token and updates OCM to match. Refresh this page after running.
							</p>
						</div>
					</div>
				</div>
			{:else if gatewayStatus?.pairingNeeded}
				<div class="mb-6 bg-yellow-900/50 border border-yellow-600 rounded-lg p-4">
					<div class="flex items-start gap-3">
						<span class="text-2xl">🔐</span>
						<div class="flex-1">
							<h3 class="text-yellow-200 font-semibold">Gateway Pairing Required</h3>
							<p class="text-yellow-100/80 text-sm mt-1">
								OCM needs to be approved to connect to OpenClaw. Run this command:
							</p>
							<div class="mt-3 bg-black/40 rounded p-3 font-mono text-sm text-green-400 relative group">
								<pre class="whitespace-pre-wrap">{gatewayStatus.approveCommand}</pre>
								<button 
									on:click={() => copyCommand(gatewayStatus?.approveCommand)}
									class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity bg-gray-700 hover:bg-gray-600 px-2 py-1 rounded text-xs text-gray-300"
								>
									Copy
								</button>
							</div>
							{#if gatewayStatus.deviceId}
								<p class="text-yellow-100/60 text-xs mt-2">
									Device ID: {gatewayStatus.deviceId.slice(0, 16)}...
								</p>
							{/if}
						</div>
					</div>
				</div>
			{/if}
			<slot />
		</main>
	</div>
{/if}
