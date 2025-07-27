<script lang="ts">
	import type { PageData } from './$types';
	import ChatView from './ChatView.svelte';
    import { _ } from 'svelte-i18n';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>{data.session?.title || $_('chat.new_chat')} - EGO</title>
</svelte:head>

{#key data.session?.id ?? 'new'}
	<ChatView {data} />
{/key}