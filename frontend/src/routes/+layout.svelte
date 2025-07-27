<script lang="ts">
	import ChatSidebar from '$lib/components/ChatSidebar.svelte';
	import { Menu } from '@lucide/svelte';
	import { fly } from 'svelte/transition';
	import { page } from '$app/stores';
	import { Toaster } from 'svelte-sonner';
	import { locale } from 'svelte-i18n';
	import { browser } from '$app/environment';
	import { initAuthStore } from '$lib/stores/auth.svelte.ts';

	const { children } = $props();

	if (browser) {
		initAuthStore();
	}
	
	let showSidebar = $state(true);
	let isMobile = $state(false);

	let isAuthPage = $derived(
		$page.route.id === '/login' || $page.route.id === '/register' || $page.route.id === '/'
	);

	$effect(() => {
		if (browser && $locale) {
			localStorage.setItem('locale', $locale);
		}
	});

	$effect(() => {
		const mediaQuery = window.matchMedia('(max-width: 768px)');
		function handleResize(e: MediaQueryListEvent | MediaQueryList) {
			isMobile = e.matches;
			showSidebar = !e.matches; 
		}
		mediaQuery.addEventListener('change', handleResize);
		handleResize(mediaQuery);
		return () => {
			mediaQuery.removeEventListener('change', handleResize);
		};
	});
</script>

<Toaster position="top-right" richColors closeButton duration={5000} />

<div class="fixed inset-0 z-[-1] overflow-hidden bg-primary">
	<div class="bg-circle -right-[20vw] -top-[20vh]" style="animation-delay: -10s;"></div>
	<div class="bg-circle -left-[20vw] -bottom-[20vh]"></div>
</div>

<div class="h-screen w-full font-sans text-text-primary">
	{#if !isAuthPage}
		<div
			class="fixed top-0 left-0 h-full z-40 transition-transform duration-300 ease-in-out"
			class:translate-x-0={showSidebar}
			class:-translate-x-full={!showSidebar}
		>
			<ChatSidebar />
		</div>
		
		{#if isMobile && showSidebar}
			<div
				onclick={() => (showSidebar = false)}
				onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (showSidebar = false)}
				role="button"
				tabindex="0"
				aria-label="Закрыть меню"
				class="fixed inset-0 z-30 bg-black/60 backdrop-blur-sm"
				in:fly={{ duration: 400, opacity: 0 }}
				out:fly={{ duration: 400, opacity: 0 }}
			></div>
		{/if}

		<button
			onclick={() => (showSidebar = !showSidebar)}
			class="fixed top-3 left-3 z-50 p-2 rounded-md bg-secondary/50 backdrop-blur-sm hover:bg-tertiary transition-colors"
			title="Переключить меню"
		>
			<Menu class="w-6 h-6" />
		</button>
	{/if}

	<main
		class="relative h-full w-full transition-[padding-left] duration-300 ease-in-out"
		class:md:pl-72={showSidebar && !isMobile && !isAuthPage}
	>
		<div class="flex flex-col h-full">
			{@render children()}
		</div>
	</main>
</div>