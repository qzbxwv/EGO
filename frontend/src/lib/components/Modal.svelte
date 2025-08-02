<script lang="ts">
	import { fade } from 'svelte/transition';
	import { X } from '@lucide/svelte';

	let { show, title, onclose, children } = $props<{
		show: boolean;
		title: string;
		onclose: () => void;
		children: any;
	}>();
</script>

{#if show}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 bg-black/70 z-40 flex items-center justify-center"
		transition:fade={{ duration: 150 }}
		onclick={onclose}
		role="dialog"
		aria-modal="true"
		aria-labelledby="modal-title"
		tabindex="-1"
		onkeydown={(e) => e.key === 'Escape' && onclose()}
	>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			class="bg-secondary border border-tertiary rounded-2xl shadow-2xl shadow-accent/10 w-full max-w-lg m-4 p-6 animate-fade-in-up"
			onclick={(e) => e.stopPropagation()}
			role="document"
		>
			<div class="flex items-center justify-between mb-4">
				<h2 id="modal-title" class="text-2xl font-bold text-text-primary">{title}</h2>
				<button onclick={onclose} class="p-2 rounded-full hover:bg-tertiary transition-colors" aria-label="Закрыть модальное окно">
					<X class="w-5 h-5 text-text-secondary" />
				</button>
			</div>
			<div class="text-text-primary">
				{@render children()}
			</div>
		</div>
	</div>
{/if}