<script lang="ts">
	import type { PageData } from './$types';
	import type { ChatMessage, ChatSession, FileAttachment, FilePayload } from '$lib/types';
	import { quintOut } from 'svelte/easing';
	import { fly, slide } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import { _ } from 'svelte-i18n';
	import { BrainCircuit, Send, StopCircle, Paperclip, X, RefreshCw } from '@lucide/svelte';

	import { auth } from '$lib/stores/auth.svelte.ts';
	import { uiStore, setShowSettingsModal } from '$lib/stores/ui.svelte.ts';
	import { updateSession } from '$lib/stores/sessions.svelte.ts';
	import { api } from '$lib/api';
	import { wsStore } from '$lib/stores/websocket.svelte.ts';
    import { streamStore, startStream, endStream } from '$lib/stores/stream.svelte.ts';
	import { LOGO_URL } from '$lib/config';
	import { optimisticMessageStore } from '$lib/stores/optimistic.svelte.ts';
	
	import Modal from '$lib/components/Modal.svelte';
	import MessageBody from '$lib/components/MessageBody.svelte';
	import LanguageSwitcher from '$lib/components/LanguageSwitcher.svelte';

	let { data }: { data: PageData } = $props();

	let messages = $state<ChatMessage[]>(data.messages || []);
	let currentSession = $state<ChatSession | null>(data.session);
	let userMessageInProgress = $state<ChatMessage | null>(null);
	let currentInput = $state('');
	let customInstructionsInput = $state(data.session?.custom_instructions || '');
	let newChatInstructions = $state('');
	let chatContainer: HTMLElement | undefined = $state();
	let inputArea: HTMLTextAreaElement | undefined = $state();
	let sessionMode = $state<'fast' | 'heavy'>(data.session?.mode || 'fast');
	let attachedFiles = $state<File[]>([]);
	let lastUserMessageForRegen = $state<ChatMessage | null>(
		(data.messages || []).filter((m) => m.author === 'user').pop() || null
	);

	const optimisticMsg = optimisticMessageStore.consume();
	if (optimisticMsg && messages.length === 0) {
		messages.push(optimisticMsg);
		lastUserMessageForRegen = optimisticMsg;
	}

	if (data.session?.id && data.session.id === streamStore.sessionId && !streamStore.isDone) {
		userMessageInProgress = {
			author: 'ego',
			text: streamStore.textStream,
			id: Date.now(),
			isThinking: true
		};
	}

    let isConnecting = $derived(!wsStore.connection);

	let inProgressMessage = $derived.by(() => {
		if (!userMessageInProgress) return null;
		
		const updatedMessage = { ...userMessageInProgress };
		updatedMessage.text = streamStore.textStream;
		updatedMessage.isThinking = !streamStore.isDone;
		
		return updatedMessage;
	});

	let currentThoughtHeader = $derived(streamStore.thoughtHeader);
	
	$effect(() => {
		scrollToBottom('auto');
	});

    $effect(() => {
		const isDone = streamStore.isDone;
		if (isDone && userMessageInProgress) {
			if (inProgressMessage && inProgressMessage.text.trim()) {
				messages.push(inProgressMessage);
				lastUserMessageForRegen = messages.filter((m) => m.author === 'user').pop() || null;
			}
			userMessageInProgress = null;
		}
	});

	$effect(() => {
		if (messages.length > 0 || inProgressMessage) {
			const _ = inProgressMessage?.text; 
			scrollToBottom('smooth');
		}
	});

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

	async function sendMessage(isRegeneration = false) {
		const egoWs = wsStore.connection;
		const isInputEmpty = currentInput.trim() === '';
		const noFiles = attachedFiles.length === 0;

		if ((isInputEmpty && noFiles && !isRegeneration) || userMessageInProgress) return;
		if (!egoWs) {
			toast.error('Соединение не установлено. Пожалуйста, обновите страницу.');
			return;
		}

		startStream();

		let userMessage: ChatMessage;
		let filePayloads: FilePayload[] = [];

		if (isRegeneration && lastUserMessageForRegen) {
			userMessage = lastUserMessageForRegen;
			const lastEGOMessageIndex = messages.findLastIndex(m => m.author === 'ego');
			if (lastEGOMessageIndex !== -1) {
				messages.splice(lastEGOMessageIndex, 1);
			}
		} else {
			filePayloads = await getFilePayloads(attachedFiles);
			const currentFilesForMessage: FileAttachment[] = attachedFiles.map((f) => ({
				file_name: f.name,
				mime_type: f.type
			}));
			userMessage = {
				author: 'user',
				text: currentInput,
				id: Date.now(),
				attachments: currentFilesForMessage
			};
			
			if (!currentSession) {
				optimisticMessageStore.set(userMessage);
			}
			
			messages.push(userMessage);
			lastUserMessageForRegen = userMessage;
		}

		userMessageInProgress = {
			author: 'ego',
			text: '',
			id: Date.now() + 1,
			isThinking: true
		};
		
		const payload = {
			query: userMessage.text,
			mode: sessionMode,
			session_id: currentSession ? currentSession.id : null,
			files: filePayloads,
			custom_instructions: currentSession ? undefined : newChatInstructions || customInstructionsInput || undefined,
            is_regeneration: isRegeneration
		};
		egoWs.send(payload);

		if (!isRegeneration) {
			if (!currentSession) {
				newChatInstructions = customInstructionsInput;
			}
			currentInput = '';
			attachedFiles = [];
			if (inputArea) inputArea.style.height = 'auto';
		}
	}

	function regenerate() {
		if (userMessageInProgress || !lastUserMessageForRegen) return;
		sendMessage(true);
	}

	function stopGeneration() {
		const egoWs = wsStore.connection;
		if (!egoWs) return;
		endStream();
		userMessageInProgress = null;
		wsStore.setConnection(null);
		toast.info($_('chat.generation_stopped'));
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	}

	async function saveSettings() {
		const instructions = customInstructionsInput || '';
		if (currentSession) {
			try {
				const updatedSessionData = { custom_instructions: instructions };
				const updated = await api.patch<ChatSession>(`/sessions/${currentSession.id}`, updatedSessionData);
				if(currentSession) currentSession.custom_instructions = updated.custom_instructions;
				updateSession(updated);
				toast.success($_('toasts.instructions_saved'));
				setShowSettingsModal(false);
			} catch (error: any) {
				toast.error(error.message);
			}
		} else {
			newChatInstructions = instructions;
			toast.success($_('chat.instructions_on_new'));
			setShowSettingsModal(false);
		}
	}

	function handleFileSelect(event: Event) {
		const target = event.target as HTMLInputElement;
		if (target.files) {
			const newFiles = Array.from(target.files).slice(0, 5 - attachedFiles.length);
			if (attachedFiles.length + newFiles.length > 5) {
				toast.warning($_('chat.file_limit_warning'));
			}
			attachedFiles = [...attachedFiles, ...newFiles];
		}
	}

	function removeFile(index: number) {
		attachedFiles.splice(index, 1);
		attachedFiles = attachedFiles;
	}
</script>

<div class="flex flex-col h-full relative overflow-hidden">
	<header
		class="flex-shrink-0 sticky top-0 z-10 p-3 pl-16 border-b border-tertiary bg-primary/80 backdrop-blur-md"
	>
		<div class="hidden xs:flex items-center justify-between gap-3">
			<h2 class="text-lg font-bold truncate pr-4">{currentSession?.title || $_('chat.new_chat')}</h2>
			<div class="flex items-center gap-1 flex-shrink-0">
				<button
					onclick={() => (sessionMode = 'fast')}
					class:bg-accent={sessionMode === 'fast'}
					class:text-white={sessionMode === 'fast'}
					class="px-3 py-1.5 text-sm rounded-md hover:bg-tertiary transition-colors duration-200">Fast</button
				>
				<button
					onclick={() => (sessionMode = 'heavy')}
					class:bg-accent={sessionMode === 'heavy'}
					class:text-white={sessionMode === 'heavy'}
					class="px-3 py-1.5 text-sm rounded-md hover:bg-tertiary transition-colors duration-200">Heavy</button
				>
				<span class="w-px h-6 bg-tertiary mx-2"></span>
				<LanguageSwitcher />
			</div>
		</div>

		<div class="flex flex-col xs:hidden gap-3">
			<div class="flex items-center justify-between">
				<h2 class="text-lg font-bold truncate pr-4">{currentSession?.title || $_('chat.new_chat')}</h2>
				<LanguageSwitcher />
			</div>
			<div class="flex items-center gap-1">
				<button
					onclick={() => (sessionMode = 'fast')}
					class:bg-accent={sessionMode === 'fast'}
					class:text-white={sessionMode === 'fast'}
					class="flex-1 py-1.5 text-sm rounded-md hover:bg-tertiary transition-colors duration-200">Fast</button
				>
				<button
					onclick={() => (sessionMode = 'heavy')}
					class:bg-accent={sessionMode === 'heavy'}
					class:text-white={sessionMode === 'heavy'}
					class="flex-1 py-1.5 text-sm rounded-md hover:bg-tertiary transition-colors duration-200">Heavy</button
				>
			</div>
		</div>
	</header>

	<div bind:this={chatContainer} class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6 scroll-pb-40">
		<div class="w-full">
			{#if messages.length === 0 && !inProgressMessage}
				<div class="flex flex-col items-center justify-center h-full text-center text-text-secondary animate-fade-in-up not-prose pt-16">
					<img src={LOGO_URL} alt="EGO Logo" class="w-16 h-16 text-accent mb-2" />
					<h2 class="text-2xl font-bold text-text-primary">{$_('chat.welcome_title')}</h2>
					<p>{$_('chat.welcome_subtitle')}</p>
				</div>
			{/if}

			<div class="flex flex-col gap-y-8 w-full">
				{#each messages as msg, i (msg.id)}
					<div
						in:fly={{ y: 20, duration: 400, delay: 100, easing: quintOut }}
						class="flex flex-col w-full group"
						class:items-end={msg.author === 'user'}
					>
						<div
							class="flex items-start gap-3 w-full"
							class:max-w-4xl={msg.author === 'ego'}
							class:max-w-3xl={msg.author === 'user'}
							class:flex-row-reverse={msg.author === 'user'}
						>
							<div
								class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 mt-1"
								class:bg-accent={msg.author === 'ego'}
								class:bg-secondary={msg.author === 'user'}
							>
								{#if msg.author === 'ego'}
									<img src={LOGO_URL} alt="EGO avatar" class="w-full h-full p-1" />
								{:else}
									<span class="font-bold text-accent text-lg"
										>{auth.user?.username?.charAt(0).toUpperCase() || 'U'}</span
									>
								{/if}
							</div>

							<div class="flex flex-col gap-2 min-w-0" class:items-end={msg.author === 'user'}>
								<div class="font-bold text-text-primary text-sm">
									{msg.author === 'ego' ? 'EGO' : $_('chat.you')}
								</div>

								{#if (msg.attachments && msg.attachments.length > 0) || msg.text}
									<div
										class="rounded-xl px-3 py-2 text-base"
										class:bg-accent={msg.author === 'user'}
										class:text-white={msg.author === 'user'}
										class:bg-secondary={msg.author === 'ego'}
									>
										<div class="flex flex-col gap-3">
											{#if msg.attachments && msg.attachments.length > 0}
												<div class="flex flex-wrap gap-2" class:justify-end={msg.author === 'user'}>
													{#each msg.attachments as attachment (attachment.file_name)}
														<div
															class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium {msg.author ===
															'user'
																? 'bg-white/20'
																: 'bg-tertiary'}"
														>
															<Paperclip class="h-4 w-4 flex-shrink-0" />
															<span class="truncate">{attachment.file_name}</span>
														</div>
													{/each}
												</div>
											{/if}

											{#if msg.text}
												<MessageBody text={msg.text} />
											{/if}
										</div>
									</div>
								{/if}
							</div>
						</div>

						{#if msg.author === 'ego' && !userMessageInProgress && i === messages.length - 1}
							<div class="mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
								<button
									onclick={regenerate}
									class="p-1.5 text-text-secondary hover:text-text-primary rounded-md hover:bg-tertiary transition-colors"
									title={$_('chat.regenerate')}
								>
									<RefreshCw class="w-4 h-4" />
								</button>
							</div>
						{/if}
					</div>
				{/each}

				{#if inProgressMessage}
					<div class="flex w-full animate-fade-in-up">
						<div class="flex items-start gap-3 w-full max-w-4xl">
							<div
								class="w-8 h-8 rounded-full bg-accent flex items-center justify-center flex-shrink-0 mt-1"
							>
								<img src={LOGO_URL} alt="EGO avatar" class="w-full h-full p-1" />
							</div>

							<div class="flex flex-col gap-2 min-w-0">
								<div class="font-bold text-text-primary text-sm">EGO</div>
								<div
									class="rounded-xl px-4 py-2.5 bg-secondary min-h-[40px] flex items-center transition-all duration-300"
									class:loading-bubble-animation={!streamStore.textStream && !currentThoughtHeader}
								>
									{#if streamStore.textStream}
										<div class="text-base w-full">
											<MessageBody text={inProgressMessage.text} />
										</div>
									{:else if currentThoughtHeader}
										{#key currentThoughtHeader}
											<div
												class="flex items-center space-x-2 text-text-secondary"
												in:fly={{ y: 5, duration: 300 }}
											>
												<BrainCircuit class="w-5 h-5 flex-shrink-0 animate-pulse" />
												<span class="font-medium animate-fade-in-up">{currentThoughtHeader || '...'}</span>
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
	</div>

	<div
		class="absolute bottom-0 left-0 right-0 pt-24 bg-gradient-to-t from-primary to-transparent pointer-events-none"
	>
		<div class="w-full px-4 pb-4 pointer-events-auto">
			<div class="max-w-4xl mx-auto flex flex-col gap-2">
				{#if attachedFiles.length > 0}
					<div class="flex flex-wrap gap-2 px-2" transition:slide>
						{#each attachedFiles as file, i (file.name + i)}
							<div class="bg-tertiary text-sm px-2 py-1 rounded-md flex items-center gap-2 animate-fade-in-up">
								<span>{file.name}</span>
								<button onclick={() => removeFile(i)} class="hover:text-red-500">
									<X class="w-3 h-3" />
								</button>
							</div>
						{/each}
					</div>
				{/if}
				<div class="input-container bg-secondary border border-tertiary rounded-2xl shadow-lg">
					<label
						class="send-button hover:bg-tertiary cursor-pointer transform hover:scale-110 transition-all duration-200"
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

					<textarea
						bind:this={inputArea}
						use:autosize
						bind:value={currentInput}
						onkeydown={handleKeydown}
						placeholder={$_('chat.placeholder')}
						disabled={isConnecting || !!userMessageInProgress}
						class="input-textarea"
						rows="1"
					></textarea>

					{#if userMessageInProgress}
						<button
							onclick={stopGeneration}
							class="send-button bg-red-500 hover:bg-red-600 text-white transform hover:scale-110 transition-all duration-200"
						>
							<StopCircle class="w-5 h-5" />
						</button>
					{:else}
						<button
							onclick={() => sendMessage()}
							disabled={isConnecting || (!currentInput.trim() && attachedFiles.length === 0)}
							class="send-button bg-accent hover:bg-accent-hover disabled:bg-tertiary disabled:cursor-not-allowed text-white transform hover:scale-110 disabled:scale-100 transition-all duration-200"
						>
							<Send class="w-5 h-5" />
						</button>
					{/if}
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