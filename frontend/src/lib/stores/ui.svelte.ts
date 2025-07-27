let showSettingsModal = $state(false);

export const uiStore = {
    get showSettingsModal() { return showSettingsModal }
};

export function setShowSettingsModal(value: boolean) {
    showSettingsModal = value;
}