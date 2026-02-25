<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type PendingDevice } from '$lib/api';

	let pending: PendingDevice[] = [];
	let loading = true;
	let error = '';
	let actionInProgress: string | null = null;

	onMount(() => {
		loadDevices();
		// Poll every 5 seconds for new pairing requests
		const interval = setInterval(loadDevices, 5000);
		return () => clearInterval(interval);
	});

	async function loadDevices() {
		try {
			const result = await api.listDevices();
			pending = result.pending || [];
			error = '';
		} catch (err) {
			// Don't show error on first load - might be expected if RPC not configured
			if (!loading) {
				error = err instanceof Error ? err.message : 'Failed to load devices';
			}
		} finally {
			loading = false;
		}
	}

	async function approve(requestId: string) {
		actionInProgress = requestId;
		try {
			await api.approveDevice(requestId);
			pending = pending.filter((d) => d.requestId !== requestId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to approve device';
		} finally {
			actionInProgress = null;
		}
	}

	async function reject(requestId: string) {
		actionInProgress = requestId;
		try {
			await api.rejectDevice(requestId);
			pending = pending.filter((d) => d.requestId !== requestId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reject device';
		} finally {
			actionInProgress = null;
		}
	}

	function formatTime(ts: number): string {
		return new Date(ts).toLocaleString();
	}

	function parseUserAgent(ua: string): string {
		// Extract browser name from user agent
		if (ua.includes('Chrome')) return 'Chrome';
		if (ua.includes('Firefox')) return 'Firefox';
		if (ua.includes('Safari')) return 'Safari';
		if (ua.includes('Edge')) return 'Edge';
		return 'Browser';
	}
</script>

{#if pending.length > 0}
	<div class="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-6">
		<div class="flex items-center gap-2 mb-3">
			<span class="text-xl">🔐</span>
			<h3 class="font-medium text-amber-900">Pending Device Pairing</h3>
			<span class="bg-amber-200 text-amber-800 text-xs font-medium px-2 py-0.5 rounded-full">
				{pending.length}
			</span>
		</div>

		{#if error}
			<div class="text-red-600 text-sm mb-2">{error}</div>
		{/if}

		<div class="space-y-2">
			{#each pending as device}
				<div class="bg-white rounded border border-amber-100 p-3 flex items-center justify-between">
					<div class="flex-1">
						<div class="flex items-center gap-2">
							<span class="font-mono text-sm text-gray-600">
								{device.deviceId.slice(0, 8)}...
							</span>
							<span class="text-xs text-gray-500">
								{parseUserAgent(device.userAgent)}
							</span>
						</div>
						<div class="text-xs text-gray-400 mt-1">
							{device.origin} • {formatTime(device.createdAt)}
						</div>
					</div>
					<div class="flex gap-2">
						<button
							on:click={() => approve(device.requestId)}
							disabled={actionInProgress === device.requestId}
							class="px-3 py-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-400 text-white text-sm rounded transition-colors"
						>
							{actionInProgress === device.requestId ? '...' : 'Approve'}
						</button>
						<button
							on:click={() => reject(device.requestId)}
							disabled={actionInProgress === device.requestId}
							class="px-3 py-1 bg-red-600 hover:bg-red-700 disabled:bg-gray-400 text-white text-sm rounded transition-colors"
						>
							Reject
						</button>
					</div>
				</div>
			{/each}
		</div>

		<p class="text-xs text-amber-700 mt-3">
			A new browser is trying to connect to OpenClaw. Approve it to allow access.
		</p>
	</div>
{/if}
