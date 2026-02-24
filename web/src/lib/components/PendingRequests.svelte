<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { api, type Elevation } from '$lib/api';

	export let requests: Elevation[];

	const dispatch = createEventDispatcher();

	let approving: string | null = null;
	let denying: string | null = null;
	let selectedTtl = '30m';

	const ttlOptions = [
		{ value: '15m', label: '15 minutes' },
		{ value: '30m', label: '30 minutes' },
		{ value: '1h', label: '1 hour' },
		{ value: '2h', label: '2 hours' },
		{ value: '4h', label: '4 hours' }
	];

	async function approve(id: string) {
		approving = id;
		try {
			await api.approveRequest(id, selectedTtl);
			dispatch('action');
		} catch (e) {
			alert(e instanceof Error ? e.message : 'Failed to approve');
		} finally {
			approving = null;
		}
	}

	async function deny(id: string) {
		denying = id;
		try {
			await api.denyRequest(id);
			dispatch('action');
		} catch (e) {
			alert(e instanceof Error ? e.message : 'Failed to deny');
		} finally {
			denying = null;
		}
	}

	function formatTime(iso: string): string {
		const date = new Date(iso);
		return date.toLocaleString();
	}

	function timeAgo(iso: string): string {
		const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
		if (seconds < 60) return `${seconds}s ago`;
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
		return `${Math.floor(seconds / 86400)}d ago`;
	}
</script>

<div class="card">
	<div class="px-6 py-4 border-b border-gray-200">
		<h2 class="text-lg font-semibold text-gray-900">Pending Requests</h2>
	</div>
	<div class="divide-y divide-gray-200">
		{#each requests as request}
			<div class="p-6">
				<div class="flex items-start justify-between">
					<div class="flex-1">
						<div class="flex items-center gap-2">
							<span class="text-lg font-medium text-gray-900">{request.service}</span>
							<span class="px-2 py-0.5 text-xs font-medium bg-orange-100 text-orange-700 rounded">
								{request.scope}
							</span>
						</div>
						<p class="mt-1 text-sm text-gray-600">{request.reason || 'No reason provided'}</p>
						<p class="mt-1 text-xs text-gray-400" title={formatTime(request.requestedAt)}>
							Requested {timeAgo(request.requestedAt)}
						</p>
					</div>
					<div class="flex items-center gap-2">
						<select
							bind:value={selectedTtl}
							class="text-sm border-gray-300 rounded-md focus:ring-primary-500 focus:border-primary-500"
						>
							{#each ttlOptions as option}
								<option value={option.value}>{option.label}</option>
							{/each}
						</select>
						<button
							class="btn btn-success"
							disabled={approving === request.id}
							on:click={() => approve(request.id)}
						>
							{approving === request.id ? 'Approving...' : 'Approve'}
						</button>
						<button
							class="btn btn-danger"
							disabled={denying === request.id}
							on:click={() => deny(request.id)}
						>
							{denying === request.id ? 'Denying...' : 'Deny'}
						</button>
					</div>
				</div>
			</div>
		{/each}
	</div>
</div>
