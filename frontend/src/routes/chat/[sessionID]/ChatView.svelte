<script lang="ts">
	import type { PageData } from './$types';
	import type { ChatMessage, ChatSession, FileAttachment, FilePayload } from '$lib/types';
	import { quintOut } from 'svelte/easing';
	import { fade, fly, slide } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import { _ } from 'svelte-i18n';
	import {
		BrainCircuit,
		Send,
		StopCircle,
		Paperclip,
		X,
		RefreshCw,
		Pencil,
		Search,
		Sparkles,
		MoreHorizontal
	} from '@lucide/svelte';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';

	import { auth } from '$lib/stores/auth.svelte.ts';
	import { uiStore, setShowSettingsModal } from '$lib/stores/ui.svelte.ts';
	import { updateSession } from '$lib/stores/sessions.svelte.ts';
	import { api } from '$lib/api';
	import { wsStore } from '$lib/stores/websocket.svelte.ts';
	import {
		streamStore,
		startStream,
		endStream,
		consumeLastUserMessageLogId
	} from '$lib/stores/stream.svelte.ts';
	import { LOGO_URL } from '$lib/config';

	import Modal from '$lib/components/Modal.svelte';
	import MessageBody from '$lib/components/MessageBody.svelte';
	import LanguageSwitcher from '$lib/components/LanguageSwitcher.svelte';

	let { data } = $props<{ data: PageData }>();

	type ChatMode = 'default' | 'deeper' | 'research';

	let messages = $state<ChatMessage[]>([]);
	let currentSession = $state<ChatSession | null>(null);
	let customInstructionsInput = $state('');
	let chatMode = $state<ChatMode>('default');
	let currentInput = $state('');
	let newChatInstructions = $state('');
	let chatContainer: HTMLElement | undefined = $state();
	let inputArea: HTMLTextAreaElement | undefined = $state();
	let attachedFiles = $state<File[]>([]);
	let userHasScrolledUp = $state(false);

	let editingLogId = $state<number | null>(null);
	let editingText = $state('');
	let messageBeingRemoved = $state<number | null>(null);

	let isMobile = $state(false);
	let showInputOptions = $state(false);

	$effect(() => {
		const mediaQuery = window.matchMedia('(max-width: 425px)');
		const update = (e: MediaQueryList | MediaQueryListEvent) => (isMobile = e.matches);
		mediaQuery.addEventListener('change', update);
		update(mediaQuery);

		return () => mediaQuery.removeEventListener('change', update);
	});

	$effect(() => {
		let loadedMessages = data.messages || [];
		if (browser) {
			if (data.session) {
				const key = `optimistic_message_${data.session.id}`;
				const optimisticJSON = sessionStorage.getItem(key);
				if (optimisticJSON) {
					const optimisticMessage = JSON.parse(optimisticJSON);
					if (!loadedMessages.some((m: ChatMessage) => m.id === optimisticMessage.id)) {
						loadedMessages.push(optimisticMessage);
					}
				}
			}
			const newChatOptimisticJSON = sessionStorage.getItem('optimistic_new_chat_message');
			if (newChatOptimisticJSON) {
				const optimisticMessage = JSON.parse(newChatOptimisticJSON);
				if (!loadedMessages.some((m: ChatMessage) => m.id === optimisticMessage.id)) {
					loadedMessages.push(optimisticMessage);
				}
				sessionStorage.removeItem('optimistic_new_chat_message');
			}
		}
		messages = loadedMessages;
		currentSession = data.session;
		customInstructionsInput = data.session?.custom_instructions || '';
		userHasScrolledUp = false;
		cancelEditing();
		scrollToBottom('auto');
	});

	let isConnecting = $derived(!wsStore.connection);

	$effect(() => {
		const logUpdate = consumeLastUserMessageLogId();
		if (logUpdate) {
			const msgIndex = messages.findIndex((m: ChatMessage) => m.id === logUpdate.temp_id);
			if (msgIndex !== -1) {
				messages[msgIndex].logId = logUpdate.log_id;
				messages = messages;
				if (browser && currentSession) {
					sessionStorage.removeItem(`optimistic_message_${currentSession.id}`);
				}
			}
		}
	});

	let lastStreamedText = $state('');
	let wasStreaming = $state(false);

	$effect(() => {
		if (!streamStore.isDone) {
			wasStreaming = true;
			lastStreamedText = streamStore.textStream;
		}
		if (streamStore.isDone && wasStreaming) {
			if (lastStreamedText.trim()) {
				const isStreamingForThisSession = streamStore.sessionId === currentSession?.id;
				const isNewChatStream = currentSession === null && streamStore.sessionId !== null;
				if (isStreamingForThisSession || isNewChatStream) {
					if (!messages.some((m: ChatMessage) => m.text === lastStreamedText && m.author === 'ego')) {
						messages.push({ author: 'ego', text: lastStreamedText, id: Date.now() });
					}
				}
			} else {
				console.log('Stream ended with empty text, not adding a new message.');
			}
			wasStreaming = false;
			lastStreamedText = '';
		}
		if (!userHasScrolledUp) {
			scrollToBottom('smooth');
		}
	});

	function handleScroll() {
		if (!chatContainer) return;
		const { scrollTop, scrollHeight, clientHeight } = chatContainer;
		if (scrollHeight - scrollTop - clientHeight > 100) {
			userHasScrolledUp = true;
		} else {
			userHasScrolledUp = false;
		}
	}

	function autosize(node: HTMLTextAreaElement) {
		const updateSize = () => {
			if (!node) return;
			node.style.height = 'auto';
			const scrollHeight = node.scrollHeight;
			const maxHeight = 200;
			if (scrollHeight > maxHeight) {
				node.style.height = `${maxHeight}px`;
				node.style.overflowY = 'auto';
			} else {
				node.style.height = `${scrollHeight}px`;
				node.style.overflowY = 'hidden';
			}
		};
		node.addEventListener('input', updateSize);
		setTimeout(updateSize, 0);
		return {
			destroy() {
				node.removeEventListener('input', updateSize);
			}
		};
	}

	function scrollToBottom(behavior: 'smooth' | 'auto' = 'smooth') {
		setTimeout(() => {
			if (chatContainer) {
				chatContainer.scrollTo({ top: chatContainer.scrollHeight, behavior });
			}
		}, 50);
	}

	async function getFilePayloads(files: File[]): Promise<FilePayload[]> {
		const promises = files.map(
			(file) =>
				new Promise<FilePayload>((resolve, reject) => {
					const reader = new FileReader();
					reader.onload = (e) => {
						const base64_data = (e.target?.result as string).split(',')[1];
						resolve({ base64_data, mime_type: file.type, file_name: file.name });
					};
					reader.onerror = (error) => reject(error);
					reader.readAsDataURL(file);
				})
		);
		return Promise.all(promises);
	}

	async function sendMessage(
		isRegeneration = false,
		regenerationLogId: number | undefined = undefined
	) {
		userHasScrolledUp = false;
		const egoWs = wsStore.connection;
		const isInputEmpty = currentInput.trim() === '';
		const noFiles = attachedFiles.length === 0;

		if ((isInputEmpty && noFiles && !isRegeneration) || !streamStore.isDone) return;
		if (!egoWs) {
			toast.error('Соединение не установлено.');
			return;
		}
		if (isRegeneration && !regenerationLogId) {
			toast.error('Не найден лог для регенерации.');
			return;
		}

		startStream(currentSession ? currentSession.id : null);

		let userMessageText = currentInput;
		let filePayloads: FilePayload[] = [];
		const tempId = Date.now();
		
		if (isRegeneration) {
			toast.info('Повторная генерация ответа...');
			const lastEgoMsgIndex = messages.findLastIndex((m) => m.author === 'ego');
			if (lastEgoMsgIndex > -1) {
				messageBeingRemoved = messages[lastEgoMsgIndex].id;
				setTimeout(() => {
					messages = messages.filter((m) => m.id !== messageBeingRemoved);
					messageBeingRemoved = null;
				}, 200);
			}
			const logToRegen = messages.find((m) => m.logId === regenerationLogId);
			userMessageText = logToRegen?.text || '';
		} else {
			filePayloads = await getFilePayloads(attachedFiles);
			const currentFilesForMessage: FileAttachment[] = attachedFiles.map((f) => ({
				file_name: f.name,
				mime_type: f.type
			}));
			const messageToSend: ChatMessage = {
				author: 'user',
				text: currentInput,
				id: tempId,
				attachments: currentFilesForMessage
			};

			if (!currentSession && browser) {
				sessionStorage.setItem('optimistic_new_chat_message', JSON.stringify(messageToSend));
			} else if (currentSession && browser) {
				sessionStorage.setItem(
					`optimistic_message_${currentSession.id}`,
					JSON.stringify(messageToSend)
				);
			}
			messages = [...messages, messageToSend];
		}

		const payload = {
			query: userMessageText,
			mode: chatMode,
			session_id: currentSession ? currentSession.id : null,
			files: filePayloads,
			custom_instructions: currentSession
				? undefined
				: newChatInstructions || customInstructionsInput || undefined,
			is_regeneration: isRegeneration,
			temp_id: isRegeneration ? undefined : tempId,
			request_log_id_to_regen: isRegeneration ? regenerationLogId : undefined
		};
		egoWs.send(payload);

		if (!isRegeneration) {
			if (!currentSession) newChatInstructions = customInstructionsInput;
			currentInput = '';
			attachedFiles = [];
			if (inputArea) inputArea.style.height = 'auto';
		}
	}

	function regenerate() {
		const lastUserMessage = messages
			.slice()
			.reverse()
			.find((m) => m.author === 'user' && m.logId);
		if (!lastUserMessage || !lastUserMessage.logId) {
			toast.error('Регенерация для этого сообщения недоступна.');
			return;
		}
		sendMessage(true, lastUserMessage.logId);
	}

	function startEditing(message: ChatMessage) {
		if (!message.logId || !streamStore.isDone) return;
		editingLogId = message.logId;
		editingText = message.text;
	}

	function cancelEditing() {
		editingLogId = null;
		editingText = '';
	}

	async function saveAndRegenerate() {
		if (!editingLogId || editingText.trim() === '') return;
		const originalLogId = editingLogId;

		try {
			await api.patch(`/logs/${originalLogId}`, { query: editingText });
			toast.success('Запрос обновлен.');

			const msgIndex = messages.findIndex((m) => m.logId === originalLogId);
			if (msgIndex > -1) {
				messages[msgIndex].text = editingText;
				messages = messages;
			}

			cancelEditing();
			sendMessage(true, originalLogId);
		} catch (e: any) {
			toast.error(`Ошибка обновления: ${e.message}`);
		}
	}

	function stopGeneration() {
		const currentStream = streamStore.textStream;
		endStream();
		if (currentStream.trim()) {
			messages.push({
				author: 'ego',
				text: currentStream,
				id: Date.now()
			});
		}
		toast.info($_('chat.generation_stopped'));
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	}

	function handleEditKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			saveAndRegenerate();
		}
		if (e.key === 'Escape') {
			cancelEditing();
		}
	}

	async function saveSettings() {
		const instructions = customInstructionsInput || '';
		if (currentSession) {
			try {
				const updatedSessionData = { custom_instructions: instructions };
				const updated = await api.patch<ChatSession>(
					`/sessions/${currentSession.id}`,
					updatedSessionData
				);
				if (currentSession) currentSession.custom_instructions = updated.custom_instructions;
				updateSession(updated);
				toast.success($_('toasts.instructions_saved'));
				setShowSettingsModal(false);
			} catch (error: any) {
				toast.error(error.message);
			}
		} else {
			newChatInstructions = instructions;
			toast.success($_('toasts.instructions_saved'));
			setShowSettingsModal(false);
		}
	}

	function handleFileSelect(event: Event) {
		const target = event.target as HTMLInputElement;
		if (target.files) {
			const newFiles = Array.from(target.files).slice(0, 5 - attachedFiles.length);
			if (attachedFiles.length + newFiles.length > 5) {
				toast.warning('Можно прикрепить не более 5 файлов.');
			}
			attachedFiles = [...attachedFiles, ...newFiles];
		}
	}

	function removeFile(index: number) {
		attachedFiles.splice(index, 1);
		attachedFiles = attachedFiles;
	}

	function toggleMode(mode: ChatMode) {
		if (chatMode === mode) {
			chatMode = 'default';
		} else {
			chatMode = mode;
		}
	}

	let lastUserMessageIndex = $derived(messages.findLastIndex((m) => m.author === 'user'));
</script>

<div class="flex flex-col h-full relative overflow-hidden">
	<header
		class="flex-shrink-0 p-3 pl-16 border-b border-tertiary bg-primary/80 backdrop-blur-md z-10"
	>
		<div class="flex items-center justify-between gap-3">
			<h2 class="text-lg font-bold truncate pr-4">{currentSession?.title || $_('chat.new_chat')}</h2>
			<div class="flex items-center gap-1 flex-shrink-0">
				<LanguageSwitcher />
			</div>
		</div>
	</header>

	<div
		bind:this={chatContainer}
		onscroll={handleScroll}
		class="flex-1 min-h-0 overflow-y-auto px-4 md:px-6 pt-6 pb-44"
	>
		{#if messages.length === 0 && streamStore.isDone}
			<div
				class="flex flex-col items-center justify-center text-center text-text-secondary animate-fade-in-up not-prose pt-16"
			>
				<img src={LOGO_URL} alt="EGO Logo" class="w-16 h-16 text-accent mb-2" />
				<h2 class="text-2xl font-bold text-text-primary">{$_('chat.welcome_title')}</h2>
				<p>{$_('chat.welcome_subtitle')}</p>
			</div>
		{/if}

		<div class="flex flex-col gap-y-4 w-full">
			{#each messages.filter((m) => m.id !== messageBeingRemoved) as msg, i (msg.id)}
				<div
					in:fly={{ y: 20, duration: 400, delay: 100, easing: quintOut }}
					out:fly={{ y: -10, duration: 200, easing: quintOut }}
					class="flex w-full group"
					class:justify-start={msg.author === 'ego'}
					class:justify-end={msg.author === 'user'}
				>
					<div
						class="flex flex-col w-fit"
						class:items-end={msg.author === 'user'}
						class:max-w-4xl={msg.author === 'ego'}
						class:max-w-3xl={msg.author === 'user'}
					>
						<div
							class="flex items-center gap-3"
							class:flex-row-reverse={msg.author === 'user'}
						>
							<div
								class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
								class:bg-accent={msg.author === 'ego'}
								class:bg-secondary={msg.author === 'user'}
							>
								{#if msg.author === 'ego'}
									<img src={LOGO_URL} alt="EGO avatar" class="w-full h-full p-1" />
								{:else}
									<span class="font-bold text-accent text-lg">{auth.user?.username?.charAt(0).toUpperCase() || 'U'}</span>
								{/if}
							</div>
							<div class="font-bold text-text-primary text-sm">
								{msg.author === 'ego' ? 'EGO' : $_('chat.you')}
							</div>
						</div>
						<div
							class="mt-1 flex flex-col gap-2"
							class:items-end={msg.author === 'user'}
							class:pl-11={msg.author === 'ego'}
							class:pr-11={msg.author === 'user'}
						>
							{#if msg.attachments && msg.attachments.length > 0 && editingLogId !== msg.logId}
								<div class="flex flex-wrap gap-2" class:justify-end={msg.author === 'user'}>
									{#each msg.attachments as attachment (attachment.file_name)}
										<div class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium {msg.author === 'user' ? 'bg-accent' : 'bg-tertiary'}">
											<Paperclip class="h-4 w-4 flex-shrink-0" />
											<span class="truncate">{attachment.file_name}</span>
										</div>
									{/each}
								</div>
							{/if}

							{#if editingLogId === msg.logId}
								<div
									transition:fly={{ y: 5, duration: 200 }}
									class="w-full flex flex-col gap-2 bg-secondary rounded-xl p-2 border border-tertiary"
								>
									<textarea
										bind:value={editingText}
										onkeydown={handleEditKeydown}
										use:autosize
										class="w-full bg-tertiary rounded-lg p-2 resize-none outline-none focus:ring-2 focus:ring-accent transition"
									></textarea>
									<div class="flex items-center gap-2 self-end">
										<button
											onclick={cancelEditing}
											class="px-3 py-1 text-sm rounded-md bg-tertiary hover:bg-tertiary/70 transition-colors"
										>{$_('sidebar.cancel_action')}</button>
										<button
											onclick={saveAndRegenerate}
											class="px-3 py-1 text-sm rounded-md bg-accent text-white hover:bg-accent-hover font-semibold transition-colors"
										>Сохранить и отправить</button>
									</div>
								</div>
							{:else if msg.text}
								<div
									class="rounded-xl px-3 py-2 text-base break-words"
									class:bg-accent={msg.author === 'user'}
									class:text-white={msg.author === 'user'}
									class:bg-secondary={msg.author === 'ego'}
								>
									<MessageBody text={msg.text} />
								</div>
							{/if}
						</div>
						<div class="mt-1 flex items-center gap-1 opacity-100 md:opacity-0 group-hover:opacity-100 transition-opacity" class:justify-end={msg.author === 'user'} class:pl-11={msg.author === 'ego'}>
							{#if streamStore.isDone && ((msg.author === 'user' && i === lastUserMessageIndex) || (msg.author === 'ego' && i === messages.length - 1))}
								<div
									transition:fly={{ y: 5, duration: 200, easing: quintOut }}
									class="bg-secondary/90 backdrop-blur-sm border border-tertiary rounded-lg shadow-lg p-1 flex items-center gap-1"
								>
									{#if msg.author === 'user'}
										<button
											onclick={() => startEditing(msg)}
											class="p-1.5 text-text-secondary hover:text-text-primary rounded-md hover:bg-tertiary/50 transition-colors"
											title="Редактировать"
										>
											<Pencil class="w-4 h-4" />
										</button>
									{/if}
									{#if msg.author === 'ego'}
										<button
											onclick={regenerate}
											class="p-1.5 text-text-secondary hover:text-text-primary rounded-md hover:bg-tertiary/50 transition-colors"
											title="Сгенерировать другой ответ"
										>
											<RefreshCw class="w-4 h-4" />
										</button>
									{/if}
								</div>
							{/if}
						</div>
					</div>
				</div>
			{/each}

			{#if !streamStore.isDone && (streamStore.sessionId === currentSession?.id || (currentSession === null && streamStore.sessionId !== null))}
				<div class="flex w-full justify-start animate-fade-in-up max-w-4xl">
					<div class="flex gap-3 w-fit items-start">
						<div class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 mt-6">
							<img src={LOGO_URL} alt="EGO avatar" class="w-full h-full p-1" />
						</div>
						<div class="flex flex-col gap-1">
							<div class="font-bold text-text-primary text-sm">EGO</div>
							<div
								class="rounded-xl px-4 py-2.5 bg-secondary min-h-[40px] min-w-[100px] flex items-center transition-all duration-300"
								class:loading-bubble-animation={!streamStore.textStream && !streamStore.thoughtHeader}
							>
								{#if streamStore.textStream}
									<div class="text-base w-full">
										<MessageBody text={streamStore.textStream} />
									</div>
								{:else if streamStore.thoughtHeader}
									{#key streamStore.thoughtHeader}
										<div
											class="flex items-center space-x-2 text-text-secondary"
											in:fly={{ y: 5, duration: 300 }}
										>
											<BrainCircuit class="w-5 h-5 flex-shrink-0 animate-pulse" />
											<span class="font-medium animate-fade-in-up"
												>{streamStore.thoughtHeader || '...'}</span
											>
										</div>
									{/key}
								{/if}
							</div>
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>

	<div
		class="absolute bottom-0 left-0 right-0 pt-24 bg-gradient-to-t from-primary to-transparent pointer-events-none"
	>
		<div class="w-full px-4 pb-4 pointer-events-auto">
			<div class="max-w-4xl mx-auto flex flex-col gap-2 relative">
				{#if showInputOptions}
                    <div
                        role="button"
                        tabindex="0"
                        onkeydown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                                showInputOptions = false;
                            }
                        }}
						class="fixed inset-0 z-10"
						onclick={() => (showInputOptions = false)}
						transition:fade={{ duration: 200 }}
					></div>
					<div
						class="absolute bottom-full mb-2 w-full bg-secondary p-2 rounded-xl shadow-lg z-20 border border-accent/30"
						transition:slide={{ duration: 150, easing: quintOut }}
					>
						<div class="flex flex-col gap-1">
							<button
								onclick={() => {
									toggleMode('deeper');
									showInputOptions = false;
								}}
								class="flex w-full items-center gap-3 p-3 text-left rounded-lg transition-colors {chatMode === 'deeper' ? 'bg-accent' : 'hover:bg-primary/50'}"
							>
								<Sparkles class="w-5 h-5" />
								<span class="font-medium">Deeper</span>
							</button>
							<button
								onclick={() => {
									toggleMode('research');
									showInputOptions = false;
								}}
								class="flex w-full items-center gap-3 p-3 text-left rounded-lg transition-colors {chatMode === 'research' ? 'bg-accent' : 'hover:bg-primary/50'}"
							>
								<Search class="w-5 h-5" />
								<span class="font-medium">Research</span>
							</button>
						</div>
					</div>
				{/if}

				{#if attachedFiles.length > 0}
					<div class="flex flex-wrap gap-2 px-2" transition:slide>
						{#each attachedFiles as file, i (file.name + i)}
							<div
								class="bg-tertiary/80 text-text-primary text-sm font-medium px-3 py-1.5 rounded-lg flex items-center gap-2.5 animate-fade-in-up"
							>
								<span class="truncate max-w-[200px]">{file.name}</span>
								<button
									onclick={() => removeFile(i)}
									class="text-text-secondary hover:text-text-primary transition-colors"
								>
									<X class="w-4 h-4" />
								</button>
							</div>
						{/each}
					</div>
				{/if}
				<div
					class="input-container bg-secondary border border-tertiary rounded-2xl shadow-lg flex flex-col"
				>
					<textarea
						bind:this={inputArea}
						use:autosize
						bind:value={currentInput}
						onkeydown={handleKeydown}
						placeholder={$_('chat.placeholder')}
						disabled={isConnecting || !streamStore.isDone || editingLogId !== null}
						class="w-full bg-transparent p-4 resize-none outline-none placeholder-text-secondary"
						rows="1"
					></textarea>
					<div
						class="flex items-center justify-between w-full border-t border-tertiary/50 px-2 py-1"
					>
						<div class="flex items-center gap-1">
							<label
								class="p-2 rounded-md hover:bg-tertiary cursor-pointer transition-colors"
								title="Прикрепить файлы"
							>
								<Paperclip class="w-5 h-5 text-text-secondary" />
								<input
									type="file"
									multiple
									onchange={handleFileSelect}
									class="hidden"
									accept="image/*,application/pdf,.txt,.md,.json,.csv"
								/>
							</label>
							{#if !isMobile}
								<button
									onclick={() => toggleMode('deeper')}
									class="flex items-center gap-1.5 px-2.5 py-1.5 text-sm rounded-md transition-colors"
									class:bg-accent={chatMode === 'deeper'}
									class:text-white={chatMode === 'deeper'}
									class:hover:bg-tertiary={chatMode !== 'deeper'}
								>
									<Sparkles class="w-4 h-4" />
									<span class="font-medium">Deeper</span>
								</button>
								<button
									onclick={() => toggleMode('research')}
									class="flex items-center gap-1.5 px-2.5 py-1.5 text-sm rounded-md transition-colors"
									class:bg-accent={chatMode === 'research'}
									class:text-white={chatMode === 'research'}
									class:hover:bg-tertiary={chatMode !== 'research'}
								>
									<Search class="w-4 h-4" />
									<span class="font-medium">Research</span>
								</button>
							{:else}
								{#if chatMode === 'deeper'}
									<button
										onclick={() => toggleMode('deeper')}
										class="flex items-center gap-1.5 px-2.5 py-1.5 text-sm rounded-md bg-accent text-white"
									>
										<Sparkles class="w-4 h-4" />
										<span class="font-medium">Deeper</span>
									</button>
								{:else if chatMode === 'research'}
									<button
										onclick={() => toggleMode('research')}
										class="flex items-center gap-1.5 px-2.5 py-1.5 text-sm rounded-md bg-accent text-white"
									>
										<Search class="w-4 h-4" />
										<span class="font-medium">Research</span>
									</button>
								{/if}
								<button
									onclick={() => (showInputOptions = !showInputOptions)}
									class="p-2 rounded-md hover:bg-tertiary cursor-pointer transition-colors"
								>
									<MoreHorizontal class="w-5 h-5 text-text-secondary" />
								</button>
							{/if}
						</div>
						<div class="flex items-center">
							{#if !streamStore.isDone}
								<button
									onclick={stopGeneration}
									class="p-2.5 rounded-full bg-red-500 hover:bg-red-600 text-white transform hover:scale-105 transition-all duration-200"
								>
									<StopCircle class="w-5 h-5" />
								</button>
							{:else}
								<button
									onclick={() => sendMessage()}
									disabled={isConnecting ||
										(!currentInput.trim() && attachedFiles.length === 0) ||
										editingLogId !== null}
									class="p-2.5 rounded-full bg-accent hover:bg-accent-hover disabled:bg-tertiary disabled:cursor-not-allowed text-white transform disabled:scale-100 transition-all duration-200"
								>
									<Send class="w-5 h-5" />
								</button>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>

	<Modal
		title={$_('chat.instructions_title')}
		show={uiStore.showSettingsModal}
		onclose={() => setShowSettingsModal(false)}
	>
		<p class="mb-4 text-text-secondary">{$_('chat.instructions_prompt')}</p>
		<textarea
			bind:value={customInstructionsInput}
			rows="5"
			class="w-full bg-primary border-2 border-tertiary rounded-lg p-3 focus:ring-accent focus:border-accent transition-all duration-300"
		></textarea>
		<button
			onclick={saveSettings}
			class="mt-4 w-full py-2 bg-accent rounded-lg font-semibold hover:bg-accent-hover transition-colors duration-200"
			>{$_('chat.save')}</button
		>
	</Modal>
</div>