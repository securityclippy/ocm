<script lang="ts">
	import { onMount } from 'svelte';

	interface PendingDevice {
		requestId: string;
		deviceId: string;
		role: string;
		origin: string;
		userAgent: string;
		createdAt: number;
	}

	interface PairedDevice {
		deviceId: string;
		role: string;
		createdAt: number;
	}

	interface DevicesResponse {
		pending: PendingDevice[];
		paired: PairedDevice[];
		error?: string;
	}

	let devices: DevicesResponse = { pending: [], paired: [] };
	let loading = true;
	let error = '';
	let actionInProgress = '';

	async function loadDevices() {
		try {
			const res = await fetch('/admin/api/devices');
			devices = await res.json();
			if (devices.error) {
				error = devices.error;
			} else {
				error = '';
			}
		} catch (e) {
			error = `Failed to load devices: ${e}`;
		} finally {
			loading = false;
		}
	}

	async function approveDevice(requestId: string) {
		actionInProgress = requestId;
		try {
			const res = await fetch(`/admin/api/devices/${requestId}/approve`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' }
			});
			if (res.ok) {
				await loadDevices();
			} else {
				const data = await res.json();
				error = data.error || 'Failed to approve device';
			}
		} catch (e) {
			error = `Failed to approve device: ${e}`;
		} finally {
			actionInProgress = '';
		}
	}

	async function rejectDevice(requestId: string) {
		actionInProgress = requestId;
		try {
			const res = await fetch(`/admin/api/devices/${requestId}/reject`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' }
			});
			if (res.ok) {
				await loadDevices();
			} else {
				const data = await res.json();
				error = data.error || 'Failed to reject device';
			}
		} catch (e) {
			error = `Failed to reject device: ${e}`;
		} finally {
			actionInProgress = '';
		}
	}

	function formatDate(timestamp: number): string {
		return new Date(timestamp).toLocaleString();
	}

	function shortenId(id: string): string {
		if (id.length <= 16) return id;
		return `${id.slice(0, 8)}…${id.slice(-8)}`;
	}

	function getTimeAgo(timestamp: number): string {
		const seconds = Math.floor((Date.now() - timestamp) / 1000);
		if (seconds < 60) return 'just now';
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
		return `${Math.floor(seconds / 86400)}d ago`;
	}

	onMount(() => {
		loadDevices();
		// Auto-refresh every 10 seconds
		const interval = setInterval(loadDevices, 10000);
		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>Devices - OCM</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Device Management</h1>
			<p class="mt-1 text-sm text-gray-500">Manage OpenClaw device pairings</p>
		</div>
		<button
			on:click={loadDevices}
			class="inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
		>
			<svg class="h-4 w-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
			</svg>
			Refresh
		</button>
	</div>

	{#if error}
		<div class="rounded-md bg-red-50 p-4">
			<div class="flex">
				<div class="flex-shrink-0">
					<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
						<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
					</svg>
				</div>
				<div class="ml-3">
					<p class="text-sm text-red-700">{error}</p>
				</div>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600"></div>
		</div>
	{:else}
		<!-- Pending Devices -->
		<div class="bg-white shadow rounded-lg">
			<div class="px-4 py-5 sm:px-6 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">
					Pending Approval
					{#if devices.pending.length > 0}
						<span class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
							{devices.pending.length}
						</span>
					{/if}
				</h2>
				<p class="mt-1 text-sm text-gray-500">Devices waiting for pairing approval</p>
			</div>
			
			{#if devices.pending.length === 0}
				<div class="px-4 py-12 text-center text-gray-500">
					<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
					<p class="mt-2">No pending device requests</p>
				</div>
			{:else}
				<ul class="divide-y divide-gray-200">
					{#each devices.pending as device}
						<li class="px-4 py-4 sm:px-6">
							<div class="flex items-center justify-between">
								<div class="flex-1 min-w-0">
									<div class="flex items-center">
										<div class="flex-shrink-0">
											<div class="h-10 w-10 rounded-full bg-yellow-100 flex items-center justify-center">
												<svg class="h-6 w-6 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
												</svg>
											</div>
										</div>
										<div class="ml-4">
											<p class="text-sm font-medium text-gray-900" title={device.deviceId}>
												{shortenId(device.deviceId)}
											</p>
											<div class="flex items-center text-sm text-gray-500">
												<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
													{device.role}
												</span>
												<span class="ml-2">{getTimeAgo(device.createdAt)}</span>
												{#if device.origin}
													<span class="ml-2">from {device.origin}</span>
												{/if}
											</div>
										</div>
									</div>
								</div>
								<div class="flex space-x-2">
									<button
										on:click={() => approveDevice(device.requestId)}
										disabled={actionInProgress === device.requestId}
										class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md shadow-sm text-white bg-green-600 hover:bg-green-700 disabled:opacity-50"
									>
										{#if actionInProgress === device.requestId}
											<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
												<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
												<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
											</svg>
										{/if}
										Approve
									</button>
									<button
										on:click={() => rejectDevice(device.requestId)}
										disabled={actionInProgress === device.requestId}
										class="inline-flex items-center px-3 py-1.5 border border-gray-300 text-xs font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50"
									>
										Reject
									</button>
								</div>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- Paired Devices -->
		<div class="bg-white shadow rounded-lg">
			<div class="px-4 py-5 sm:px-6 border-b border-gray-200">
				<h2 class="text-lg font-medium text-gray-900">
					Paired Devices
					{#if devices.paired.length > 0}
						<span class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
							{devices.paired.length}
						</span>
					{/if}
				</h2>
				<p class="mt-1 text-sm text-gray-500">Approved devices with access to OpenClaw</p>
			</div>
			
			{#if devices.paired.length === 0}
				<div class="px-4 py-12 text-center text-gray-500">
					<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
					</svg>
					<p class="mt-2">No paired devices</p>
				</div>
			{:else}
				<ul class="divide-y divide-gray-200">
					{#each devices.paired as device}
						<li class="px-4 py-4 sm:px-6">
							<div class="flex items-center">
								<div class="flex-shrink-0">
									<div class="h-10 w-10 rounded-full bg-green-100 flex items-center justify-center">
										<svg class="h-6 w-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
										</svg>
									</div>
								</div>
								<div class="ml-4 flex-1">
									<p class="text-sm font-medium text-gray-900" title={device.deviceId}>
										{shortenId(device.deviceId)}
									</p>
									<div class="flex items-center text-sm text-gray-500">
										<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
											{device.role}
										</span>
										{#if device.createdAt}
											<span class="ml-2">paired {getTimeAgo(device.createdAt)}</span>
										{/if}
									</div>
								</div>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>
