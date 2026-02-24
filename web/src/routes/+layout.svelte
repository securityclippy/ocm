<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
</script>

<div class="min-h-screen flex">
	<Sidebar />
	<main class="flex-1 p-8">
		<slot />
	</main>
</div>
