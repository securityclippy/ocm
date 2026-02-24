<script lang="ts">
	import type { AuditEntry } from '$lib/api';

	export let entries: AuditEntry[];

	function formatTime(iso: string): string {
		const date = new Date(iso);
		return date.toLocaleString();
	}

	function getActionColor(action: string): string {
		if (action.includes('approved')) return 'text-green-600 bg-green-50';
		if (action.includes('denied') || action.includes('revoked')) return 'text-red-600 bg-red-50';
		if (action.includes('request')) return 'text-orange-600 bg-orange-50';
		if (action.includes('access')) return 'text-blue-600 bg-blue-50';
		return 'text-gray-600 bg-gray-50';
	}

	function formatAction(action: string): string {
		return action.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
	}
</script>

<div class="card">
	<div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
		<h2 class="text-lg font-semibold text-gray-900">Recent Activity</h2>
		<a href="/audit" class="text-sm text-primary-600 hover:text-primary-700">View all</a>
	</div>
	<div class="divide-y divide-gray-200">
		{#each entries as entry}
			<div class="px-6 py-4 flex items-center gap-4">
				<div class="flex-shrink-0">
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium {getActionColor(entry.action)}">
						{formatAction(entry.action)}
					</span>
				</div>
				<div class="flex-1 min-w-0">
					<div class="text-sm text-gray-900">
						{#if entry.service}
							<span class="font-medium">{entry.service}</span>
							{#if entry.scope}
								<span class="text-gray-500">:{entry.scope}</span>
							{/if}
						{:else}
							<span class="text-gray-500">System</span>
						{/if}
					</div>
					{#if entry.details}
						<p class="text-xs text-gray-500 truncate">{entry.details}</p>
					{/if}
				</div>
				<div class="flex-shrink-0 text-xs text-gray-400">
					{formatTime(entry.timestamp)}
				</div>
				<div class="flex-shrink-0 text-xs text-gray-500">
					{entry.actor}
				</div>
			</div>
		{/each}
	</div>
</div>
