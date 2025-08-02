<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { _ } from 'svelte-i18n';
	import { LOGO_URL } from '$lib/config';
</script>

<div class="flex-1 flex flex-col items-center justify-center p-8 text-center">
	<img src={LOGO_URL} alt="EGO Logo" class="w-16 h-16 mb-4 text-accent" />
	<h1 class="text-3xl font-bold text-text-primary">{$page.status}: Чат не найден</h1>
	<p class="mt-2 text-text-secondary">
		{$page.error?.message.includes('Unexpected token')
			? 'Кажется, чата по этому адресу не существует.'
			: $page.error?.message}
	</p>
	<button
		onclick={() => goto('/chat/new')}
		class="mt-6 px-6 py-3 bg-accent text-white font-semibold rounded-lg shadow-lg hover:bg-accent-hover transition-colors focus:outline-none focus:ring-2 focus:ring-accent focus:ring-opacity-50"
	>
		Начать новый чат
	</button>
</div>