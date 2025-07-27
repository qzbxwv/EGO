<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { MessageSquare, Plus, Trash2, Settings, LogOut } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { _ } from 'svelte-i18n';

	import { auth, logout } from '$lib/stores/auth.svelte.ts';
	import { sessionStore, fetchSessions, removeSession } from '$lib/stores/sessions.svelte.ts';
	import { uiStore, setShowSettingsModal } from '$lib/stores/ui.svelte.ts';
	import { api } from '$lib/api';
	import { LOGO_URL } from '$lib/config';

	function confirmDelete(sessionId: number, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();

		toast.error($_('sidebar.delete_confirm'), {
			action: {
				label: $_('sidebar.delete_action'),
				onClick: async () => {
					try {
						const isCurrentSession = $page.params.sessionID === String(sessionId);
						await api.delete(`/sessions/${sessionId}`);
						toast.success($_('sidebar.session_deleted'));

						removeSession(sessionId);
						
						if (isCurrentSession) {
							await goto('/chat/new', { invalidateAll: true });
						}
					} catch (e: any) {
						toast.error(e.message || 'Ошибка удаления сессии');
					}
				}
			},
			cancel: {
				label: $_('sidebar.cancel_action'),
				onClick: () => {}
			}
		});
	}
	
	function newChat() {
		goto('/chat/new', { invalidateAll: true });
	}

	$effect(() => {
		if (auth.user) {
			fetchSessions();
		}
	});
</script>

<aside class="w-72 bg-secondary/80 backdrop-blur-lg flex flex-col p-4 border-r border-tertiary h-full">
	<div class="flex items-center justify-between mb-4 flex-shrink-0">
		<div class="w-10 h-10"></div>

		<a href="/chat" class="flex items-center gap-2 text-2xl font-bold text-text-primary">
			<img src={LOGO_URL} alt="EGO Logo" class="w-8 h-8" />
			EGO
		</a>

		<button
			onclick={newChat}
			class="p-2 rounded-lg hover:bg-tertiary/50 transition-colors w-10 h-10 flex items-center justify-center"
			aria-label={$_('chat.new_chat')}
		>
			<Plus class="w-6 h-6" />
		</button>
	</div>

	<div class="flex-1 overflow-y-auto -mr-2 pr-2 min-h-0">
		<nav class="space-y-1">
			{#if sessionStore.isLoading}
				{#each Array(5) as _}
					<div class="h-10 p-3 bg-tertiary/50 rounded-lg animate-pulse"></div>
				{/each}
			{:else if sessionStore.sessions.length === 0}
				<p class="text-text-secondary text-sm px-2 py-4 text-center">
					{$_('sidebar.no_sessions')}
				</p>
			{:else}
				{#each sessionStore.sessions as session (session.id)}
					<a
						href="/chat/{session.id}"
						class="flex items-center justify-between p-3 rounded-lg text-sm group transition-colors duration-200 {$page.params.sessionID === String(session.id)
							? 'bg-accent text-white'
							: 'hover:bg-tertiary/50'}"
					>
						<div class="flex items-center space-x-3 truncate">
							<MessageSquare class="w-4 h-4 flex-shrink-0" />
							<span class="truncate font-medium">{session.title}</span>
						</div>
						<button
							onclick={(e) => confirmDelete(session.id, e)}
							class="p-1 rounded-md text-text-secondary hover:bg-red-500/50 hover:text-white opacity-0 group-hover:opacity-100 transition-all focus:opacity-100"
							aria-label={`Удалить сессию ${session.title}`}
						>
							<Trash2 class="w-4 h-4" />
						</button>
					</a>
				{/each}
			{/if}
		</nav>
	</div>

	<div class="mt-auto pt-4 border-t border-tertiary flex-shrink-0">
		{#if auth.user}
			<div class="space-y-1 mb-2">
				<button
					onclick={() => setShowSettingsModal(true)}
					class="flex w-full items-center gap-3 p-3 rounded-lg text-sm text-text-secondary hover:bg-tertiary/50 transition-colors duration-200"
				>
					<Settings class="w-4 h-4" />
					<span>{$_('chat.instructions_title')}</span>
				</button>
			</div>
			<div class="flex items-center justify-between p-2">
				<span class="text-sm text-text-secondary truncate font-medium" title={auth.user?.username}>
					{auth.user?.username}
				</span>
				<button
					onclick={logout}
					class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-sm font-medium text-red-400 hover:bg-red-500/20 hover:text-red-300 transform hover:scale-105 transition-all duration-200"
				>
					<LogOut class="w-4 h-4" />
					<span>{$_('sidebar.logout')}</span>
				</button>
			</div>
		{/if}
	</div>
</aside>