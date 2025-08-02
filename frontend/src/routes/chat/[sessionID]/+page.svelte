<script lang="ts">
	import type { PageData } from './$types';
	import { _ } from 'svelte-i18n';
	import ChatView from './ChatView.svelte';

	let { data } = $props<{ data: PageData }>();
</script>

<svelte:head>
	<title>{data.session?.title || $_('chat.new_chat')} - EGO</title>
</svelte:head>

{#key data.session?.id ?? 'new'}
	<ChatView {data} />
{/key}