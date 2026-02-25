<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import SetupWizard from '$lib/components/SetupWizard.svelte';

	let setupComplete = true; // Default to true to avoid flash
	let loading = true;

	onMount(async () => {
		try {
			const status = await api.getSetupStatus();
			setupComplete = status.setupComplete;
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
			<slot />
		</main>
	</div>
{/if}
