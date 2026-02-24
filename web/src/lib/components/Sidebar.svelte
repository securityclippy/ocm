<script lang="ts">
	import { page } from '$app/stores';

	const navItems = [
		{ href: '/', label: 'Dashboard', icon: '📊' },
		{ href: '/credentials', label: 'Credentials', icon: '🔑' },
		{ href: '/requests', label: 'Requests', icon: '📋' },
		{ href: '/audit', label: 'Audit Log', icon: '📜' }
	];

	$: currentPath = $page.url.pathname;
</script>

<aside class="w-64 bg-gray-900 text-white flex flex-col">
	<div class="p-6">
		<h1 class="text-xl font-bold flex items-center gap-2">
			<span class="text-2xl">🔐</span>
			<span>OCM</span>
		</h1>
		<p class="text-xs text-gray-400 mt-1">Credential Manager</p>
	</div>

	<nav class="flex-1 px-4">
		<ul class="space-y-1">
			{#each navItems as item}
				<li>
					<a
						href={item.href}
						class="flex items-center gap-3 px-4 py-2.5 rounded-lg transition-colors {currentPath === item.href
							? 'bg-primary-600 text-white'
							: 'text-gray-300 hover:bg-gray-800 hover:text-white'}"
					>
						<span>{item.icon}</span>
						<span>{item.label}</span>
					</a>
				</li>
			{/each}
		</ul>
	</nav>

	<div class="p-4 border-t border-gray-700">
		<div class="text-xs text-gray-400">
			OCM v0.1.0
		</div>
	</div>
</aside>
